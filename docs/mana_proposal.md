# Mana Implementation Proposal

The goal of this document is to provide a high level overview of how mana will be implemented in GoShimmer.

## Introduction

Mana is a a reputation system for nodes within the IOTA network.

Reputation is gained by contributing to the network, i.e. creating value transfers.
Reputation is lost when not cooperating with the network: using up all the available bandwidth, advocating for
"bad" transfers during voting, etc..

As time passes, earned mana of a node decays to encourage keeping up the good behavior.

## Scope

The sope of the first implementation of mana into GoShimmer is to verify that mana calculations work,
study mana distribution in the test network and verify that nodes have similar view on the network. A test dApp could
be written to see how nodes react to "simulated misbehaving" nodes.

## Mana Calculation

Mana is esentially the reputation score of a node in the IOTA network. Mana is calculated locally in each node, as a
function that takes value transactions as input and produces the Base Mana Vector as output.
It consists of Base Mana 1 and Base Mana 2. Given a value transaction, these are determined as follows:
 1. Base Mana 1 is revoked from the node that created the output(s) used as input(s) in the transaction, and is pledged to
    the node creating the new output(s).
 2. Base Mana 2 is freshly created at the issuance time of the transaction, awarded to the node, but decays with time.

Access and Consensus Mana (mana used by other node modules) are derived from Base Mana 1 and 2 and "smoothened" through
exponential moving averages. Since Base Mana 2 decays, it is important to find a suitable interval at which the value
is updated in code. Same goes for Access and Consensus Mana,
as EMAs are also time dependent.

The exact mathematical formulas, and their respective parameters will be determined later.

## Mana Plugin in GoShimmer

Mana should be implemented as a separate plugin in GoShimmer.

The `mana plugin` is responsible for:
 - calculating mana from value transactions,
 - keeping a log of the different mana values of all nodes,
 - updating mana values in a timely fashion,
 - respond to mana related queries from other modules,
 - allowing certain modules to overwrite mana of nodes (punishing bad behavior).

 The proposed mana plugin in should keep track of the different mana values of nodes and handle calculation and timely
 updates. Mana values are mapped to `nodeID`s and stored in a `map` data structure:
 ```go
baseMana1Map := map[<nodeID>[]byte]<baseMana1-value>float64, and so on...
```
 Base Mana 2, EMAs, Consensus and Access Mana records are updated every `t` interval with tickers in a dedicated `BackgrounWorker`.

 Since the different manas can be updated from other modules and goroutines (punish misbehaving nodes), the maps should be protected with synchronization
 primitives.

 The mana plugin should expose utility funtions to other modules for:
  1. Obtaining the list of N highest mana nodes.
  2. Obtaining the mana value of a particular node.
  3. Obtaining a list of currently known peers + their mana, sorted. Useful for knowing which high mana nodes are online.
  4. Obtaining a list of neighbors and their mana.
  5. Decreasing (overwriting) some form of mana because of detected misbehaving.
  6. Obtaining the full mana maps for comparison with the perception of other nodes.

  +1, consider adding new API routes for mana related infos.

 Such utility functions could be used for example to visualize mana distribution in node dashboard, or send neighbor
 mana data to the analysis server for further processing.

## Challenges

### Dependency on Value Tangle

Since mana is awarded to nodes submitting value transfers, the value tangle is needed as input for mana calculation.
Each node calculates mana locally, therefore, it is essential to determine when to consider transactions in the value
tangle "final enough" (so that they will not be orphaned). If such a mechanism was given , mana awarding could be tied
to this condition. Eventually, this mechanism will be snapshotting, but in the first iterations of mana prototyping in
goshimmer, we can't rely on this.

For goshimmer mana prototyping, the following are the options for mana awarding trigger condition:
 - value transaction becomes solid
 - value transaction has at least X direct/indirect approvers
 - value transaction is directly/indirectly referenced by all tips (basically snapshot condition?)
 - certain time points. For example, calculate new mana every 30th minute in an hour. Problematic because of different
   local clocks and timestamps can't be trusted until they are voted upon.

### Value Transaction Layout

When processing value transactions in the value layer for mana calculation, either the original message containing the
value payload should be revisited to determine the correct `nodeID`, or a new field should be added to `ValuePayload`/`Transaction`
denoting `nodeID`. The latter is also beneficial to implement mana donation feature, that is, to donate the mana of a
certain transaction ot an arbitrary node.

## Limitations

The first implementation of mana in goshimmer will:
  - not have voted timestamps on value transactions,
  - lack proper condition to trigger mana update,
  - lack integration into rate control/autopeering/fpc/etc.
