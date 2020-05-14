# Filecoin (go-filecoin)

[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg)](https://circleci.com/gh/filecoin-project/go-filecoin)
[![User Devnet Release](https://img.shields.io/endpoint.svg?color=brightgreen&style=flat&logo=GitHub&url=https://raw.githubusercontent.com/filecoin-project/go-filecoin-badges/master/user-devnet.json)](https://github.com/filecoin-project/go-filecoin/releases/latest)
[![Nightly Devnet Release](https://img.shields.io/endpoint.svg?color=blue&style=flat&logo=GitHub&url=https://raw.githubusercontent.com/filecoin-project/go-filecoin-badges/master/nightly-devnet.json)](https://github.com/filecoin-project/go-filecoin/releases)
[![Staging Devnet Release](https://img.shields.io/endpoint.svg?color=brightgreen&style=flat&logo=GitHub&url=https://raw.githubusercontent.com/filecoin-project/go-filecoin-badges/master/staging-devnet.json)](https://github.com/filecoin-project/go-filecoin/releases)

The original Filecoin node implementation in Go.

__Questions or problems with go-filecoin? [Ask the community first](#community)__. Your problem may already be solved.

## Table of Contents
<!--
    TOC generated by https://github.com/thlorenz/doctoc
    Install with `npm install -g doctoc`.
    Regenerate with `doctoc README.md`.
    It's ok to edit manually if you don't have/want doctoc.
 -->
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [What is Filecoin?](#what-is-filecoin)
- [Install](#install)
  - [System Requirements](#system-requirements)
  - [Install from Source](#install-from-source)
    - [Install Go](#install-go)
    - [Install Dependencies](#install-dependencies)
    - [Build and run tests](#build-and-run-tests)
- [Usage](#usage)
  - [Advanced usage](#advanced-usage)
    - [Setting up a localnet](#setting-up-a-localnet)
- [Contributing](#contributing)
- [Community](#community)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## What is Filecoin?
Filecoin is a decentralized storage network that turns the world’s unused storage into an algorithmic market, creating a permanent, decentralized future for the web.
**Miners** earn the native protocol token (also called “Filecoin”) by providing data storage and/or retrieval. 
**Clients** pay miners to store or distribute data and to retrieve it.
Check out the [Filecoin home page](https://filecoin.io/) and [documentation site](https://docs.filecoin.io/) for more.

## Install

👋 Welcome to Go Filecoin!

This README outlines the basics for building and running Go-filecoin.
**For more background, configuration, and troubleshooting information check out the [Go-filecoin docs](https://go.filecoin.io/)**.

### System Requirements

Go-filecoin can build and run on most Linux and MacOS systems. 
Windows is not yet supported.

A validating node can run on most systems with at least 8GB of RAM. 
A mining node requires significant RAM and GPU resources, depending on the sector configuration to be used.

### Install from Source

Clone this git repository to your machine:

```sh
mkdir -p /path/to/filecoin-project
git clone https://github.com/filecoin-project/go-filecoin.git /path/to/filecoin-project/go-filecoin
```

#### Install Go

The build process for go-filecoin requires [Go](https://golang.org/doc/install) >= v1.13

> Installing Go for the first time? We recommend [this tutorial](https://www.ardanlabs.com/blog/2016/05/installing-go-and-your-workspace.html) which includes environment setup.

Due to our use of `cgo`, you'll need a C compiler to build go-filecoin whether you're using a prebuilt library or building it yourself from source. 
If you want to use `gcc` (e.g. `export CC=gcc`) when building go-filecoin, you will need to use v7.4.0 or higher.

The build process will download a static library containing the [Filecoin proofs implementation](https://github.com/filecoin-project/rust-fil-proofs) (which is written in Rust).
If instead you wish to build it from source, by setting the environment variable `FFI_BUILD_FROM_SOURCE=1`, you'll need a Rust development environment too.
More info at [filecoin-ffi](https://github.com/filecoin-project/filecoin-ffi).

#### Install Dependencies

First, to load all the Git submodules.

```sh
git submodule update --init --recursive
```

Initialize build dependencies.

```sh
make deps
```

Note: The first time you run `deps` can be **slow** as very large parameter files are either downloaded or generated locally in `/var/tmp/filecoin-proof-parameters`.
Have patience; future runs will be faster.

#### Build and run tests

```sh
# First, build the binary
make

# Then, run the unit tests.
go run ./build test

# Build and test can be combined!
go run ./build best
```

Other handy build commands include:

```sh
# Check the code for style and correctness issues
go run ./build lint

# Run different categories of tests by toggling their flags
go run ./build test -unit=false -integration=true -functional=true

# Test with a coverage report
go run ./build test -cover

# Test with Go's race-condition instrumentation and warnings (see https://blog.golang.org/race-detector)
go run ./build test -race

# Deps, Lint, Build, Test (any args will be passed to `test`)
go run ./build all
```

Note: Any flag passed to `go run ./build test` (e.g. `-cover`) will be passed on to `go test`.

If you have **problems with the build**, please consult the [Go-filecoin docs](https://go.filecoin.io/) site.

## Usage

The [Go-filecoin docs](https://go.filecoin.io/) site contains a tutorial sequence to guide you through
running a Filecoin node and performing some basic storage operations. 

To see a full list of commands, run `./go-filecoin --help`.

A quick start for connecting to an existing network:

```sh
# Remove any existing symlink to a repo directory
rm ~/.filecoin

# Initialize a new repository, downloading a genesis file and setting network parameters for the "interop" developer network
./go-filecoin init --genesisfile=https://gateway.ipfs.io/ipfs/QmZ7ENWU2TYBuRfdXRjoEFvF6mNpYzyALPHexcgnonUeoK/interop.car --devnet-interop

# Run the daemon
./go-filecoin daemon
```

Your node should connect to some peers and begin downloading and validating the blockchain.

You can interact with your node by running the `go-filecoin` binary again in a different terminal.

```sh
# Print the node's connection information
./go-filecoin id

# Connect to a peer
./go-filecoin swarm connect <peerid>

# Show chain sync status
./go-filecoin chain status
```

### Advanced usage

#### Setting up a localnet

The localnet FAST binary tool allows users to quickly and easily setup a local network on the users computer. 
Please refer to the [localnet README](https://github.com/filecoin-project/go-filecoin/tree/master/tools/fast/bin/localnet#localnet) for more information. 
The localnet tool is only compatible when built from the same git ref as the targeted `go-filecoin` binary.

## Contributing

We ❤️ all our contributors; this project wouldn’t be what it is without you! If you want to help out, please see [CONTRIBUTING.md](CONTRIBUTING.md).

Check out the [Go-Filecoin code overview](CODEWALK.md) for a brief tour of the code.

## Community

Here are a few places to get help and connect with the Filecoin community:
- [Go-filecoin Documentation](http://go.filecoin.io/) — for tutorials, troubleshooting, and FAQs
- The `#fil-go-filecoin` channel on [Filecoin Project Slack](https://filecoinproject.slack.com/messages/CEHHJNJS3/) or [Matrix/Riot](https://riot.im/app/#/room/#fil-dev:matrix.org) - for live help and some dev discussions
- [Filecoin Community Forum](https://discuss.filecoin.io) - for talking about design decisions, use cases, implementation advice, and longer-running conversations
- [GitHub issues](https://github.com/filecoin-project/go-filecoin/issues) - to report bugs, and view or contribute to ongoing development.
- [Filecoin Specification](https://github.com/filecoin-project/specs) - how Filecoin is supposed to work

Looking for even more? See the full rundown at [filecoin-project/community](https://github.com/filecoin-project/community).

## License

The Filecoin Project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/filecoin-project/go-filecoin/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/filecoin-project/go-filecoin/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
