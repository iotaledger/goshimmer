# GoShimmer

![GitHub Workflow Status](https://img.shields.io/github/workflow/status/iotaledger/goshimmer/Build?style=for-the-badge) ![GitHub release (latest by date)](https://img.shields.io/github/v/release/iotaledger/goshimmer?style=for-the-badge) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/iotaledger/goshimmer?style=for-the-badge)

## Table of Content
1. [Motivation](#Motivation)
2. [Design](#Design)
3. [Modules overview](#Modules-overview)
4. [Run GoShimmer](#Run-GoShimmer)
5. [Configure GoShimmer](#Configure-GoShimmer)
6. [How to contribute](#How-to-contribute)

## Motivation

This repository is where the IOTA Foundation's Research Team experiments and runs simulations of the Coordicide modules to study and evaluate their performance.
Even though the development of this code is ongoing and hence not finished, we want to give the community the opportunity to follow the development process closely, take part in the testing of the individual modules and learn more about how it works.

## Design

GoShimmer is designed in a modular fashion, where each module represents one of the essential [Coordicide components](https://coordicide.iota.org/) as well as core components necessary to work as a fullnode (e.g. gossip layer, ledger state, API).  

![alt text](images/building-blocks.png "Coordicide blueprint")

This approach enables to convert the concepts piece-by-piece and more importantly, simultaneous but independent of each other, into a prototype.
At its core, GoShimmer is based on a `event-driven` approach. We typically define the logic of each module within the folder `packages` and we use the `plugins` folder to enable the node to use a given module, thus, accordingly changing its behavior.
You can have a look at the `main.go` file to see which plugins are currently supported. 

## Modules overview

The `master` branch allows to run a GoShimmer node with a preliminary set of components for enabling `data-transactions`.
You can find more details about our roadmap [here](https://roadmap.iota.org/).

In the following, we describe some of the modules currently implemented. 

> Please note that not all the modules are currently integrated. 
When they are not, they are typically kept on a different branch. 
These branches are not compatible with the `master` one (i.e., nodes running code of these branches will not be able to be part of the current network).
This is beacuse either the code is still on a highly experimental state or different breaking changes are required to be addressed before merging (e.g, atomic transactions, UTXO, binary support). 

You can also find some libraries that are shared with [Hornet](https://github.com/gohornet/hornet) by checking out the [hive.go](https://github.com/iotaledger/hive.go) repository.
If you would like to know more about the other modules, just have a look at the code.

### Nodes identity
Each node creates a unique public/private key pair. The public key is used to identify nodes during autopeering. In the future, these identities will allow nodes to receive mana.

### Autopeering 
Autopeering is a mechanism that allows nodes to choose their neighbors automatically. More specifically, each new node on the network tries to connect to four neighbors (chosen neighbors) and 
accepts connections from other four neighbors (accepted neighbors). We describe how it works in our Autopeering blogposts [part-1](https://blog.iota.org/coordicide-update-autopeering-part-1-fc72e21c7e11) and [part-2](https://blog.iota.org/coordicide-update-autopeering-part-2-4e462ba68bd). 
We also provide a standalone autopeering simulator [here](https://github.com/iotaledger/autopeering-sim), that uses the exact same code we run on GoShimmer.

### Web-API

For more information about these API, you can refer to [swagger-link]()

### Ledger State

The branch `ledger state` implements a first version of the [Parallel-reality](https://iota.cafe/t/parallel-reality-based-ledger-state-using-utxo/261) -based ledger state (using the UTXO model). 

![parallel_reality](images/outputs.png "Ledger State")

### Work in progress research topics

#### Rate control

Currently, PoW is used to prevent spam. We are working on an `Adaptive-PoW` mechanism described in the [Coordicide-WP](https://coordicide.iota.org/) that we will integrate in a future release.
Moreover, we are experimenting via simulations on an `Additive Increase Multilpicative Decrease (AIMD)`-based approach for the rate control. You can find the initial source code at this [repository](https://github.com/andypandypi/IOTARateControl). 

#### Mana

The branch `mana` contains a first implementation of `mana` as described in the [Coordicide-WP](https://coordicide.iota.org/). Currently, only the package is provided.

#### Cellular Consensus 

The branch `ca` contains a first implementation of the `Cellular Consensus` as described in the [Coordicide-WP](https://coordicide.iota.org/).

#### Fast Probabilistic Consensus

The branch `fpc` contains a first implementation of the `Fast Probabilistic Consensus` as described in Popov et al. [paper](https://arxiv.org/pdf/1905.10895.pdf). 
You can also find a standalone FPC simulator [here](https://github.com/iotaledger/fpc-sim).

## Run GoShimmer

You have three options to run GoShimmer:
* via the binary
* compiling from the source code

### Run the binary

Linux/MacOSX
```
./goshimmer
```

Windows
```
goshimmer.exe
```

### Compile from source code

#### Prerequisites

First, you need to [install Go](https://golang.org/doc/install) if it is not already installed on your machine. It is recommended that you use the most recent version of Go.

To verify that you have installed the minimal required Go version (1.13) run:

```
go version
``` 

#### Build

1. Clone the repository

```
git clone git@github.com:iotaledger/goshimmer.git
```

or if you prefer https over ssh

```
git clone https://github.com/iotaledger/goshimmer.git
```

2. You can build your executable (as well as cross compiling for other architectures) by running the `go build` tool inside the just cloned folder `goshimmer`:

```
go build -o goshimmer (or goshimmer.exe)
```

3. You can then run by:

Linux/MacOSX
```
./goshimmer
```

Windows
```
goshimmer.exe
```

## Configure GoShimmer

GoShimmer supports configuring the exposed services (e.g., changing address and ports) as well as the enabled/disabled plugins. 

There are two ways you can configure GoShimmer:

* via a configuration file (`config.json`)
* via command line

### Command line

For a list of all the available configuration parameters you can run:

```
./goshimmer --help
```

You can then override the parameters of the `config.json` by using these options.

#### (Optional) Install Glumb visualizer

You're developing on GoShimmer and have checked out the repository:
```
(in the root folder)
git submodule init
git submodule update
```

You've downloaded a binary only:
```bash
(in the root folder)
git clone https://github.com/glumb/IOTAtangle.git
// only this version seems to be stable
cd IOTAtangle && git reset --hard 07bba77a296a2d06277cdae56aa963abeeb5f66e 
cd ../
git clone https://github.com/socketio/socket.io-client.git
```

In both cases make sure to either define the `Graph` plugin as enabled in `config.json` or via CLI (`--node.enablePlugins="Graph"`)

## How to contribute

1. Clone the repository.
2. Create a new branch for your fix or feature `git checkout -b fix/my-fix or feat/my-feat`.
3. Make sure that your code is properly formatted with `go fmt` and documentation is written for exported members of packages.
4. Target your PR for to be merged with `dev`.
