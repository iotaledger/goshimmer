<h1 align="center">
  <br>
  <a href="https://goshimmer.docs.iota.org/goshimmer.html"><img src="images/GoShimmer.png"></a>
</h1>

<h2 align="center">Prototype node software for an IOTA network without the Coordinator</h2>

<p align="center">
    <a href="https://goshimmer.docs.iota.org/goshimmer.html" style="text-decoration:none;">
    <img src="https://img.shields.io/badge/Documentation%20portal-blue.svg?style=for-the-badge" alt="Developer documentation portal">
</p>
<p align="center">
  <a href="https://discord.iota.org/" style="text-decoration:none;"><img src="https://img.shields.io/badge/Discord-9cf.svg?logo=discord" alt="Discord"></a>
    <a href="https://iota.stackexchange.com/" style="text-decoration:none;"><img src="https://img.shields.io/badge/StackExchange-9cf.svg?logo=stackexchange" alt="StackExchange"></a>
    <a href="https://github.com/iotaledger/goshimmer/blob/master/LICENSE" style="text-decoration:none;"><img src="https://img.shields.io/github/license/iotaledger/goshimmer.svg" alt="Apache 2.0 license"></a>
    <a href="https://golang.org/doc/install" style="text-decoration:none;"><img src="https://img.shields.io/github/go-mod/go-version/iotaledger/goshimmer" alt="Go version"></a>
    <a href="" style="text-decoration:none;"><img src="https://img.shields.io/github/v/release/iotaledger/goshimmer" alt="Latest release"></a>
</p>
      
<p align="center">
  <a href="#about">About</a> ◈
  <a href="#design">Design</a> ◈
  <a href="#getting-started">Getting started</a> ◈
  <a href="#client-library-and-http-api-reference">Client-Library and HTTP API reference</a> ◈
  <a href="#supporting-the-project">Supporting the project</a> ◈
  <a href="#joining-the-discussion">Joining the discussion</a> 
</p>

---

## About

This repository, called GoShimmer, is where the IOTA Foundation's Research Department tests the IOTA 2.0 modules to study and evaluate their performance.

GoShimmer is first and foremost a research prototype. As such, braking changes can often happen. We invite researchers and developers to make use of this project as you see fit. Running experiments, test out new ideas, build PoC are all very welcome initiatives.

For a documentation, including tutorials and resources, we refer to the [Documentation](http://goshimmer.docs.iota.org/).

## Design
The code in GoShimmer is modular, where each module represents either one of the *IOTA 2.0 components* or a basic node function such as the gossip, ledger state, and API - just to mention a few.  

![Layers](docs/protocol_specification/layers.png)

GoShimmer's modularity is based on a combination of event-driven and layer-based approaches.

## Client-Library and HTTP API reference

You can use the Go client-library to interact with GoShimmer (located under `github.com/iotaledger/goshimmer/client`).

You can find more info about this on our [client-lib](https://goshimmer.docs.iota.org/apis/api.html) and [Web API](https://goshimmer.docs.iota.org/apis/webAPI.html) GitHub page.

## Getting started

You can find tutorials on how to [setup a GoShimmer node](https://goshimmer.docs.iota.org/tutorials/setup.html), [writing a dApp](https://goshimmer.docs.iota.org/tutorials/dApp.html), [obtaining tokens from the faucet](https://goshimmer.docs.iota.org/tutorials/request_funds.html) and more on our [GitHub Page](https://goshimmer.docs.iota.org/goshimmer.html).

### Compiling from source

We always recommend to run your node via [Docker](https://goshimmer.docs.iota.org/tutorials/setup.html). However, you can also compile the source and run the node from the compiled binary. GoShimmer uses [RocksDB](https://github.com/linxGnu/grocksdb) as its underlying db engine. That requires a few dependencies before building the project: 
- librocksdb
- libsnappy
- libz
- liblz4
- libzstd 

Please follow this guide: https://github.com/facebook/rocksdb/blob/master/INSTALL.md to build above libs.

Finally, when compiling GoShimmer, just run the build script:

```bash
./scripts/build.sh
```

If you also want to link the libraries statically (only on Linux) run this instead:
```bash
./scripts/build_goshimmer_rocksdb_builtin.sh
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
