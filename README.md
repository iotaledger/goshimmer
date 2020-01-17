<h1 align="center">
  <br>
  <a href="https://docs.iota.org/docs/node-software/0.1/goshimmer/introduction/overview.md"><img src="images/GoShimmer.png"></a>
</h1>

<h2 align="center">Prototype node software for an IOTA network without the Coordinator</h2>

<p align="center">
    <a href="https://docs.iota.org/docs/node-software/0.1/goshimmer/introduction/overview.md" style="text-decoration:none;">
    <img src="https://img.shields.io/badge/Documentation%20portal-blue.svg?style=for-the-badge" alt="Developer documentation portal">
</p>
<p align="center">
  <a href="https://discord.iota.org/" style="text-decoration:none;"><img src="https://img.shields.io/badge/Discord-9cf.svg?logo=discord" alt="Discord"></a>
    <a href="https://iota.stackexchange.com/" style="text-decoration:none;"><img src="https://img.shields.io/badge/StackExchange-9cf.svg?logo=stackexchange" alt="StackExchange"></a>
    <a href="https://github.com/iotaledger/goshimmer/blob/master/LICENSE" style="text-decoration:none;"><img src="https://img.shields.io/github/license/iotaledger/goshimmer.svg" alt="Apache 2.0 license"></a>
    <a href="https://golang.org/doc/install" style="text-decoration:none;"><img src="https://img.shields.io/github/go-mod/go-version/iotaledger/goshimmer" alt="Go version"></a>
    <a href="" style="text-decoration:none;"><img src="https://img.shields.io/github/workflow/status/iotaledger/goshimmer/Build" alt="Build status"></a>
    <a href="" style="text-decoration:none;"><img src="https://img.shields.io/github/v/release/iotaledger/goshimmer" alt="Latest release"></a>
</p>
      
<p align="center">
  <a href="#about">About</a> ◈
  <a href="#design">Design</a> ◈
  <a href="#implemented-coordicide-modules">Implemented Coordicide modules</a> ◈
  <a href="#work-in-progress-modules">Work-in-progress modules</a> ◈
  <a href="#installation">Installation</a> ◈
  <a href="#getting-started">Getting started</a> ◈
  <a href="#api-reference">API reference</a> ◈
  <a href="#supporting-the-project">Supporting the project</a> ◈
  <a href="#joining-the-discussion">Joining the discussion</a> 
</p>

---

## About

This repository is where the IOTA Foundation's Research Department runs simulations of the Coordicide modules to study and evaluate their performance.

The aim of this open repository is  to give the community the opportunity to follow developments, take part in testing, and learn  more about [Coordicide](https://coordicide.iota.org/).

**Note:** You can find details about future development plans in our [roadmap](https://roadmap.iota.org).

## Design

The code in GoShimmer is modular, where each module represents either one of the [Coordicide components](https://coordicide.iota.org/) or a basic node function such as the gossip layer, ledger state, and API.  

![Coordicide blueprint](images/building-blocks.png)

This approach allows us to develop each module in parallel and to test GoShimmer with one or more different versions.

Each module is defined in the `packages` directory and can be enabled, using the `plugins` directory.

**Note:** See the `main.go` file to see which plugins are currently supported.

## Implemented Coordicide modules

The `master` branch is the stable version of the GoShimmer software, which includes a minimal set of modules to allow you to send and gossip zero-value transactions.

The `master` branch includes the following Coordicide modules: 

- [Node identities](https://coordicide.iota.org/module1)

- [Autopeering](https://coordicide.iota.org/module2)


The autopeering module is divided into two submodules:

- **Peer discovery:** Responsible for operations such as discovering new peers and verifying their online status

- **Neighbor selection:** Responsible for finding and managing neighbors

![autopeering](images/autopeering.png "Autopeering design")

## Work-in-progress modules

Work-in-progress modules are typically kept on a different branch such as `mana`, and are not compatible with the `master` branch. Therefore, nodes that run these branches cannot join the current network because the code either is still too experimental or it includes breaking changes.

The following Coordicide modules are a work in progress: 

- [Mana](https://coordicide.iota.org/module1): The `mana` branch contains a first implementation of the mana module in the `packages` directory.

- [Cellular Consensus](https://coordicide.iota.org/module5.1.1): The `ca` branch contains a first implementation of the Cellular Consensus module in the `packages` directory.

- [Fast Probabilistic Consensus](https://coordicide.iota.org/module5.1.2): The `fpc` branch contains a first implementation of the Fast Probabilistic Consensus  module in the `packages` directory. You can also find a standalone FPC simulator [here](https://github.com/iotaledger/fpc-sim).

- [Spam Protection](https://coordicide.iota.org/module3): You can find the initial source code in this [repository](https://github.com/andypandypi/IOTARateControl).

As well as these modules, we are working on the following node functions:

- Ledger State: The `ledger_state` branch implements a version of the [parallel-reality-based ledger state](https://iota.cafe/t/parallel-reality-based-ledger-state-using-utxo/261) (using the UTXO model). 

    ![parallel_reality](images/outputs.png "Ledger State")

## API reference

For a full list of API commands for GoShimmer, go to the [our Swagger page]().

## Installation

You have two options to install and run GoShimmer:

- Use the precompiled executable file
- Compile the code from source

### Execute the precompiled executable file

This repository includes a `goshimmer` file (for Linux and macOS) and a `goshimmer.exe` file (for Windows), which are precompiled executables.

To run the node, all you need to do is execute one of these files, depending on your operating system.

```bash
# Linux and macOS
./goshimmer
# Windows
goshimmer.exe
```

### Compile the code from source

If you want to build your own executable file, you need to follow these steps.

#### Prerequisites

To complete this guide, you need to [have at least version 1.13 of Go](https://golang.org/doc/install)installed on your device.

To check if you have Go installed, run the following command:

```bash
go version
```

If Go is installed, you should see the version that's installed.

---

1. Clone the repository

    ```bash
    git clone https://github.com/iotaledger/goshimmer.git
    ```

2. Change into the `goshimmer` directory

3. Use one of the following commands to build your executable file, depending on your operating system

    ```bash
    # Linux and macOS
    go build -o goshimmer
    # Windows
    go build -o  goshimmer.exe
    ```

    **Note:** If you're using Windows PowerShell, enclose `goshimmer.exe` in single quotation marks. For example: go build -o 'goshimmer.exe'.

## Getting started

When you first run GoShimmer, the node starts running and tries to connects to neighbors, using the autopeering module.

To run other modules such as `spammer` or `zeromq`, you can configure GoShimmer to enable them through plugins.

**Note:** For a list of all the available configuration parameters, you can run the following command:

```bash
# Linux and macOS
./goshimmer -help
# Windows
goshimmer.exe -help
```

You can configure GoShimmer in the following ways:

* Use a configuration file called `config.json`
* Use command-line options

The repository includes a `config.json` file, which the executable file will find and use when you execute it.

To use the command line, execute the file with one of the following commands, depending on your operating system

```bash
# Linux and macOS
./goshimmer --node.enablePlugins "spammer zeromq dashboard"
# Windows
goshimmer.exe --node.enablePlugins "spammer zeromq dashboard"
```

Here, we use the command-line flags to enable the spammer, ZMQ, and dashboard plugins. These plugins allow you to send spam transactions to your node, monitor it for incoming transactions, and view the total number of transactions that it's processing in a web dashboard.

### Installing the Glumb visualizer

The Glumb visualiser allows you to view the transactions in the network, using a web browser.

1. Enable the `Graph` plugin either in your `config.json` file or in the command line (`--node.enablePlugins="Graph"`)

2. If you're running GoShimmer with the precompiled executable file, do the following:

    ```bash
    (in the root folder)
    git clone https://github.com/glumb/IOTAtangle.git
    // only this version seems to be stable
    cd IOTAtangle && git reset --hard 07bba77a296a2d06277cdae56aa963abeeb5f66e 
    cd ../
    git clone https://github.com/socketio/socket.io-client.git
    ```

3. If you built the code from source, do the following in the `goshimmer` directory:

    ```bash
    git submodule init
    git submodule update
    ```

## Supporting the project

If you want to contribute to the code, consider posting a [bug report](https://github.com/iotaledger/goshimmer/issues/new-issue), feature request or a [pull request](https://github.com/iotaledger/goshimmer/pulls/).

When creating a pull request, we recommend that you do the following:

1. Clone the repository
2. Create a new branch for your fix or feature. For example, `git checkout -b fix/my-fix` or ` git checkout -b feat/my-feature`.
3. Run the `go fmt` command to make sure your code is well formatted
4. Document any exported packages
5. Target your pull request to be merged with `dev`

## Joining the discussion

If you want to get involved in the community, need help getting started, have any issues related to the repository or just want to discuss blockchain, distributed ledgers, and IoT with other people, feel free to join our [Discord](https://discord.iota.org/).
