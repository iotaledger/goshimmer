---
description: GoShimmer comes with a docker private network, integration tests and a CLI wallet to test the stability of the protocol.
image: /img/logo/goshimmer_light.png
keywords:
- tools
- docker
- private network
- integration test
- cli
- wallet
- cli wallet
- dags visualizer
---
# Tooling

GoShimmer comes with some tools to test the stability of the protocol.

We provide a documentation for the following tools:

- The [docker private network](docker_private_network.md) with which a local test network can be set up locally with docker.
- The [integration tests](integration_tests.md) spins up a `tester` container within which every test can specify its own GoShimmer network with Docker.
- The [cli-wallet](../tutorials/wallet_library.md) is described as part of the tutorial section.
- The [DAGs Visualizer](dags_visualizer.md) is the all-round tool for visualizing DAGs.
- The [rand-seed and rand-address](rand_seed_and_rand_address.md) to randomly generate a seed, with the relative public key, or a random address.