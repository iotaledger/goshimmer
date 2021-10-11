---
description: Frequently Asked Questions. What is GoShimmer?,  What Kind of Confirmation Time Can I Expect?, Where Can I See the State of the GoShimmer testnet?,How Many Transactions Per Second(TPS) can GoShimmer Sustain?, How is Spamming Prevented?, What Happens if I Issue a Double Spend?, Who's the Target Audience for Operating a GoShimmer Node?
image: /img/logo/goshimmer_light.png
keywords:
- average network delay
- testnet
- analysis
- dashboard
- vote
- frequently asked questions
- node software
- double spend
- transactions
---
# FAQ

## What is GoShimmer?

GoShimmer is a research and engineering project from the IOTA Foundation seeking to evaluate Coordicide concepts by implementing them in a node software.

## What Kind of Confirmation Time Can I Expect?

Since non-conflicting transactions aren't even voted on, they materialize after 2x the average network delay parameter we set. This means that a transaction usually confirms within a time boundary of ~10 seconds.

## Where Can I See the State of the GoShimmer testnet?

You can access the global analysis dashboard in the [Pollen Analyzer](http://analysisentry-01.devnet.shimmer.iota.cafe:28080/) which showcases the network graph and active ongoing votes on conflicts.

## How Many Transactions per Second (TPS) Can GoShimmer Sustain?

The transactions per second metric is irrelevant for the current development state of GoShimmer. We are evaluating components from Coordicide, and aren't currently interested in squeezing out every little ounce of performance. Since the primary goal is to evaluate Coordicide components, we value simplicity over optimization . Even if we would put out a TPS number, it would not reflect an actual metric in a finished production ready node software. 

## How is Spamming Prevented?

The Coordicide lays out concepts for spam prevention through the means of rate control and such. However, in the current version, GoShimmer relies on Proof of Work (PoW) to prevent over saturation of the network. Doing the PoW for a message will usually take a couple of seconds on commodity hardware.

## What Happens if I Issue a Double Spend?

If issue simultaneous transactions spending the same funds, there is high certainty that your transaction will be rejected by the network. This rejection will block your funds indefinitely, though this may change in the future.  

If you issue a transaction, await the average network delay, and then issue the double spend, then the first issued transaction should usually become confirmed, and the 2nd one rejected.  

## Who's the Target Audience for Operating a GoShimmer Node?

Our primary focus is testing out Coordicide components. We are mainly interested in individuals who have a strong IT background, rather than giving people of any knowledge-level the easiest way to operate a node. We welcome people interested in trying out the bleeding edge of IOTA development and providing meaningful feedback or problem reporting in form of [issues](https://github.com/iotaledger/goshimmer/issues/new/choose).