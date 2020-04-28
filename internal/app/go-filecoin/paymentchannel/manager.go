package paymentchannel

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initActor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	paychActor "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	xerrors "github.com/pkg/errors"
	"github.com/prometheus/common/log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

var defaultGasPrice = types.NewAttoFILFromFIL(actor.DefaultGasCost)
var defaultGasLimit = gas.NewGas(5000)
var zeroAmt = abi.NewTokenAmount(0)

// Manager manages payment channel actor and the data paymentChannels operations.
type Manager struct {
	ctx             context.Context
	paymentChannels *statestore.StateStore
	sender          MsgSender
	waiter          MsgWaiter
	stateViewer     ActorStateViewer
}

// PaymentChannelStorePrefix is the prefix used in the datastore
var PaymentChannelStorePrefix = "/retrievaldeals/paymentchannel"

// MsgWaiter is an interface for waiting for a message to appear on chain
type MsgWaiter interface {
	Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error
}

// MsgSender is an interface for something that can post messages on chain
type MsgSender interface {
	// Send posts a message to the chain
	Send(ctx context.Context,
		from, to address.Address,
		value types.AttoFIL,
		gasPrice types.AttoFIL,
		gasLimit gas.Unit,
		bcast bool,
		method abi.MethodNum,
		params interface{}) (out cid.Cid, pubErrCh chan error, err error)
}

// ActorStateViewer is an interface to StateViewer that the Manager uses
type ActorStateViewer interface {
	GetStateView(ctx context.Context, tok shared.TipSetToken) (ManagerStateView, error)
}

// NewManager creates and returns a new paymentchannel.Manager
func NewManager(ctx context.Context, ds datastore.Batching, waiter MsgWaiter, sender MsgSender, viewer ActorStateViewer) *Manager {
	store := statestore.New(namespace.Wrap(ds, datastore.NewKey(PaymentChannelStorePrefix)))
	return &Manager{ctx, store, sender, waiter, viewer}
}

// AllocateLane adds a new lane to a payment channel entry
func (pm *Manager) AllocateLane(paychAddr address.Address) (laneID uint64, err error) {
	err = pm.paymentChannels.
		Get(paychAddr).
		Mutate(func(info *ChannelInfo) error {
			laneID = info.NextLane
			info.NextLane++
			info.NextNonce++
			return nil
		})
	return laneID, err
}

// GetPaymentChannelByAccounts looks up a payment channel via payer/payee
// returns an empty ChannelInfo if not found.
func (pm *Manager) GetPaymentChannelByAccounts(payer, payee address.Address) (*ChannelInfo, error) {
	var chinfos []ChannelInfo
	var found ChannelInfo

	if err := pm.paymentChannels.List(&chinfos); err != nil {
		return nil, err
	}
	for _, chinfo := range chinfos {
		if chinfo.From == payer && chinfo.To == payee {
			found = chinfo
			break
		}
	}
	return &found, nil
}

// GetPaymentChannelInfo retrieves channel info from the paymentChannels.
// Assumes channel exists.
func (pm *Manager) GetPaymentChannelInfo(paychAddr address.Address) (*ChannelInfo, error) {
	storedState := pm.paymentChannels.Get(paychAddr)
	if storedState == nil {
		return nil, xerrors.New("no stored state")
	}
	var chinfo ChannelInfo
	if err := storedState.Get(&chinfo); err != nil {
		return nil, err
	}
	return &chinfo, nil
}

// CreatePaymentChannel will send the message to the InitActor to create a paych.Actor.
// If successful, a new payment channel entry will be persisted to the
// paymentChannels via a message wait handler.  Returns the created payment channel address
func (pm *Manager) CreatePaymentChannel(client, miner address.Address, amt abi.TokenAmount) (address.Address, cid.Cid, error) {
	errReturn := func(err error) (address.Address, cid.Cid, error) {
		return address.Undef, cid.Undef, err
	}

	chinfo, err := pm.GetPaymentChannelByAccounts(client, miner)
	if err != nil {
		return errReturn(err)
	}
	if !chinfo.IsZero() {
		return errReturn(xerrors.Errorf("payment channel exists for client %s, miner %s", client, miner))
	}

	execParams, err := PaychActorCtorExecParamsFor(client, miner)
	if err != nil {
		return errReturn(err)
	}

	mcid, _, err := pm.sender.Send(
		pm.ctx,
		client,
		builtin.InitActorAddr,
		types.NewAttoFIL(amt.Int),
		defaultGasPrice,
		defaultGasLimit,
		true,
		builtin.MethodsInit.Exec,
		&execParams,
	)
	if err != nil {
		return errReturn(err)
	}
	go pm.handlePaychCreateResult(pm.ctx, mcid, client, miner)
	return address.Undef, mcid, nil
}

// AddVoucherToChannel saves a new signed voucher entry to the payment store
// Assumes paychAddr channel has already been created.
// Called by retrieval client connector
func (pm *Manager) AddVoucherToChannel(paychAddr address.Address, voucher *paychActor.SignedVoucher) error {
	return pm.saveNewVoucher(paychAddr, voucher, nil)
}

// AddVoucher saves voucher to the store
// If payment channel record does not exist in store, it will be created.
// Each new voucher amount must be > the last largest voucher by at least `expected`
// Called by retrieval provider connector
func (pm *Manager) AddVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte, expected big.Int, tok shared.TipSetToken) (abi.TokenAmount, error) {
	has, err := pm.ChannelExists(paychAddr)
	if err != nil {
		return zeroAmt, err
	}
	if !has {
		return pm.createPaymentChannelWithVoucher(paychAddr, voucher, proof, tok)
	}

	chinfo, err := pm.GetPaymentChannelInfo(paychAddr)
	if err != nil {
		return zeroAmt, err
	}
	// check that this voucher amount is sufficiently larger than the last, largest voucher amount.
	largest := chinfo.LargestVoucherAmount()
	delta := abi.TokenAmount{Int: abi.NewTokenAmount(0).Sub(voucher.Amount.Int, largest.Int)}
	if expected.LessThan(delta) {
		return zeroAmt, xerrors.Errorf("voucher amount insufficient")
	}
	if err = pm.saveNewVoucher(paychAddr, voucher, proof); err != nil {
		return zeroAmt, err
	}

	return delta, nil
}

// ChannelExists returns whether paychAddr has a store entry, + error
// Exported for retrieval provider
func (pm *Manager) ChannelExists(paychAddr address.Address) (bool, error) {
	return pm.paymentChannels.Has(paychAddr)
}

// PaychActorCtorExecParamsFor constructs parameters to send a message to InitActor
// To construct a paychActor
func PaychActorCtorExecParamsFor(client, miner address.Address) (initActor.ExecParams, error) {

	ctorParams := paychActor.ConstructorParams{From: client, To: miner}
	marshaled, err := encoding.Encode(ctorParams)
	if err != nil {
		return initActor.ExecParams{}, err
	}

	p := initActor.ExecParams{
		CodeCID:           builtin.PaymentChannelActorCodeID,
		ConstructorParams: marshaled,
	}
	return p, nil
}

// GetMinerWorkerAddress mocks getting a miner worker address from the miner address
func (pm *Manager) GetMinerWorkerAddress(ctx context.Context, miner address.Address, tok shared.TipSetToken) (address.Address, error) {
	view, err := pm.stateViewer.GetStateView(ctx, tok)
	if err != nil {
		return address.Undef, err
	}
	_, fcworker, err := view.MinerControlAddresses(ctx, miner)
	return fcworker, err
}

func (pm *Manager) WaitForCreatePaychMessage(ctx context.Context, mcid cid.Cid) (address.Address, error) {
	var newPaychAddr address.Address

	handleResult := func(b *block.Block, sm *types.SignedMessage, mr *vm.MessageReceipt) error {
		var res initActor.ExecReturn
		if err := encoding.Decode(mr.ReturnValue, &res); err != nil {
			return err
		}

		var msgParams initActor.ExecParams
		if err := encoding.Decode(sm.Message.Params, &msgParams); err != nil {
			return err
		}

		var ctorParams *paychActor.ConstructorParams
		if err := encoding.Decode(msgParams.ConstructorParams, &ctorParams); err != nil {
			return err
		}

		newPaychAddr = res.RobustAddress
		return nil
	}

	err := pm.waiter.Wait(pm.ctx, mcid, handleResult)
	if err != nil {
		return address.Undef, err
	}
	return newPaychAddr, nil
}

func (pm *Manager) AddFundsToChannel(paychAddr address.Address, amt abi.TokenAmount) (cid.Cid, error) {
	var chinfo ChannelInfo
	st := pm.paymentChannels.Get(paychAddr)
	if err := st.Get(&chinfo); err != nil {
		return cid.Undef, err
	}

	mcid, _, err := pm.sender.Send(context.TODO(), chinfo.From, paychAddr, amt, defaultGasPrice, defaultGasLimit, true, builtin.MethodSend, nil)
	if err != nil {
		return cid.Undef, err
	}
	// TODO: track amts in paych store
	return mcid, nil
}

func (pm *Manager) WaitForAddFundsMessage(ctx context.Context, mcid cid.Cid) error {
	handleResult := func(b *block.Block, sm *types.SignedMessage, mr *vm.MessageReceipt) error {
		if mr.ExitCode != exitcode.Ok {
			return xerrors.Errorf("Add funds failed with exitcode %d", mr.ExitCode)
		}
		return nil
	}
	return pm.waiter.Wait(pm.ctx, mcid, handleResult)
}

// WaitForPaychCreateMsg waits for mcid to appear on chain and returns the robust address of the
// created payment channel
// TODO: paych store locking, wait outside store lock, also set up channel tracking somehow
// before knowing paych addr
func (pm *Manager) handlePaychCreateResult(ctx context.Context, mcid cid.Cid, client, miner address.Address) {

	handleResult := func(_ *block.Block, _ *types.SignedMessage, mr *vm.MessageReceipt) error {
		if mr.ExitCode != 0 {
			log.Errorf("create message failed with exit code %d", mr.ExitCode)
		}

		var decodedReturn initActor.ExecReturn
		if err := decodedReturn.UnmarshalCBOR(bytes.NewReader(mr.ReturnValue)); err != nil {
			return err
		}
		paychAddr := decodedReturn.RobustAddress

		// TODO check again to make sure a payment channel has not been created for this From/To
		chinfo := ChannelInfo{
			From:       client,
			To:         miner,
			NextLane:   0,
			NextNonce:  1,
			UniqueAddr: paychAddr,
		}
		return pm.paymentChannels.Begin(paychAddr, &chinfo)
	}

	if err := pm.waiter.Wait(ctx, mcid, handleResult); err != nil {
		log.Errorf("payment channel creation failed because: %w", err)
	}
}

// Called ONLY in context of a retrieval provider.
func (pm *Manager) createPaymentChannelWithVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte, tok shared.TipSetToken) (abi.TokenAmount, error) {
	view, err := pm.stateViewer.GetStateView(pm.ctx, tok)
	if err != nil {
		return zeroAmt, err
	}
	from, to, err := view.PaychActorParties(pm.ctx, paychAddr)
	if err != nil {
		return zeroAmt, err
	}
	// needs to "allocate" a lane as well as storing a voucher so this bumps
	// lane once and nonce twice
	chinfo := ChannelInfo{
		From:       from,
		To:         to,
		NextLane:   1,
		NextNonce:  2,
		UniqueAddr: paychAddr,
		Vouchers:   []*VoucherInfo{{Voucher: voucher, Proof: proof}},
	}
	if err = pm.paymentChannels.Begin(paychAddr, &chinfo); err != nil {
		return zeroAmt, err
	}
	return voucher.Amount, nil
}

// saveNewVoucher saves a voucher to an existing payment channel
func (pm *Manager) saveNewVoucher(paychAddr address.Address, voucher *paychActor.SignedVoucher, proof []byte) error {
	var chinfo ChannelInfo
	st := pm.paymentChannels.Get(paychAddr)
	if err := st.Get(&chinfo); err != nil {
		return err
	}
	if chinfo.NextLane <= voucher.Lane {
		return xerrors.Errorf("lane does not exist %d", voucher.Lane)
	}
	if chinfo.HasVoucher(voucher) {
		return xerrors.Errorf("voucher already saved")
	}
	if err := pm.paymentChannels.
		Get(paychAddr).
		Mutate(func(info *ChannelInfo) error {
			info.NextNonce++
			info.Vouchers = append(info.Vouchers, &VoucherInfo{
				Voucher: voucher,
				Proof:   proof,
			})
			return nil
		}); err != nil {
		return err
	}
	return nil
}
