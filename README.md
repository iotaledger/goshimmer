# goshimmer

[![Build Status](https://travis-ci.org/iotaledger/goshimmer.svg?branch=master)](https://travis-ci.org/iotaledger/goshimmer)

## Motivation

This repository is where the IOTA Foundation's Research Team experiments and run simulations of the Coordicide modules to study and evaluate their performance.
Even though the development of this code is ongoing and hence not finished, we want to give the community the opportunity to follow the development process closely, take part in the testing of the individual modules and learn more about how it works.

## Design

GoShimmer is  designed  in  a  modular  fashion,  where  each  module  represents one of the essential [Coordicide’s components](https://coordicide.iota.org/) as well as core components necessary  to  work  as  a  full-node  (e.g.,  gossip  layer,  ledger  state,  API).  

![alt text](images/building-blocks.png "Coordicide blueprint")


This approach enables to convert the concepts piece-by-piece and more importantly, simultaneous but independent of each other, into a prototype.

## Modules overview

The `master` branch allows to run a GoShimmer node with a preliminary set of components for enabling `data-transactions`.

When all the modules become available, the GoShimmer nodes will become the `Coordicide-testnet`, which is a release candidate for the next IOTA protocol. You can find more details about our `roadmap` [here](https://roadmap.iota.org/).

In the following, we describe some of the modules currently implemented. If you would like to know more about the other modules, just have a look at the code.

### Nodes identity
Each node creates a unique public/private key pair. The public key is used to identify nodes during auto-peering. In the future, these identities will allow nodes to receive mana.

### Autopeering 
The autopeering is a mechanism that allows nodes to choose their neighbors automatically. More specifically, each new node on the network tries to connect to four neighbors (chosen neighbors) and accepts connections from other four neighbors (accepted neighbors). We describe how it works in our Autopeering blogposts [part-1](https://blog.iota.org/coordicide-update-autopeering-part-1-fc72e21c7e11) and [part-2](https://blog.iota.org/coordicide-update-autopeering-part-2-4e462ba68bd). 
We also provide a standalone autopeering simulator at this [repository](https://github.com/iotaledger/autopeering-sim), that uses the exactly same code we run on GoShimmer.

### Web-API
GoShimmer currently provides the following web-API:

* `broadcastData`: allows to broadcat `data-transactions`

* `getTrytes`: returns the raw trytes of transactions

* `getTransactions`: returns the json objects of transactions

* `findTransactions`: returns all the transaction hashes for the given addresses

* `getNeighbors`: returns a json object arrays of the connected neighbors, split into two arrays named chosen and accepted

For more information about these API, you can refer to [swagger-link]()

### Ledger State

The branch `ledger state` implements a first version of the[Parallel-reality](https://iota.cafe/t/parallel-reality-based-ledger-state-using-utxo/261)-based ledger state (using the UTXO model). 

### Rate control

Currently, PoW is used to prevent spam. We are working on a `Adaptive-PoW` mechanism described in the [Coordicide-WP](https://coordicide.iota.org/) that we will integrate in a future release. Moreover, we are experimenting via simulations an `Additive Increase Multilpicative Decrease (AIMD)`-based approach for the rate control. You can find the initial source code at this [repository](https://github.com/andypandypi/IOTARateControl). 

### Mana

The branch `mana` contains a first implementation of `mana` as described in in the [Coordicide-WP](https://coordicide.iota.org/). Currently, only the package is provided

### Cellular Consensus 

The branch `ca` contains a first implementation of the `Cellular Consensus` as described in the [Coordicide-WP](https://coordicide.iota.org/).

### Fast Probabilistic Consensus

The branch `fpc` contains a first implementation of the `Fast Probabilistic Consensus` as described in Popov et al. [paper](https://arxiv.org/pdf/1905.10895.pdf). 
You can also find a standalone FPC simulator at this [repository](https://github.com/iotaledger/fpc-sim).


## Run GoShimmer

First, you need to [install Go](https://golang.org/doc/install) if it is not already installed on your machine. It is recommended that you use the most recent version of Go.

### Requirements

- gcc: Some packages in this repo might require to be compiled by gcc. Windows users can install [MinGW-gcc](http://tdm-gcc.tdragon.net/download). 


## Build

If you need to develop locally and be able to build by using your local code, i.e., without waiting for pushing your commits on the repo, clone the repository directly inside the `src/github.com/iotaledger/` folder of your `$GOPATH` with the command:

```
git clone git@github.com:iotaledger/goshimmer.git
```

or if you prefer https over ssh

```
git clone https://github.com/iotaledger/goshimmer.git
```

Verify that you have installed the minimal required go version (1.13):
```
go version
```

You can build your executable (as well as cross compiling for other architectures) by running the `go build` tool inside the just cloned folder `goshimmer`:

```
go build -o shimmer
```

On Windows:
```
ren shimmer shimmer.exe
```

You can then run by:

Linux
```
./shimmer
```

Windows
```
shimmer
```

## Docker

To run Shimmer on docker, you must first build the image with
```
docker build -t iotaledger/goshimmer .
```
and then run it with
```
docker run --rm -it -v "$(pwd)/mainnetdb:/app/mainnetdb" iotaledger/goshimmer
```
You may replace `$(pwd)/mainnetdb` with a custom path to the database folder.

To start Shimmer in the background, you can also simply use [Docker Compose](https://docs.docker.com/compose/) by running
```
docker-compose up -d
```

### Install Glumb visualizer

Install both the Glumb visualizer and socket.io client lib within the root folder/where the binary is located:
```bash
git clone https://github.com/glumb/IOTAtangle.git
// only this version seems to be stable
cd IOTAtangle && git reset --hard 07bba77a296a2d06277cdae56aa963abeeb5f66e 
cd ../
git clone https://github.com/socketio/socket.io-client.git
```
