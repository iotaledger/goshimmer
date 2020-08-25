# Mana Implementation Proposal

The goal of this document is to provide a high level overview of how mana will be implemented in GoShimmer.

## Introduction

Mana is a reputation system for nodes within the IOTA network.

Reputation is gained by contributing to the network, i.e. creating value transfers.
As time passes, earned mana of a node decays to encourage keeping up the good behavior.

## Scope

The sope of the first implementation of mana into GoShimmer is to verify that mana calculations work,
study base mana calculations 1 & 2, and mana distribution in the test network, furthermore to verify that nodes have
similar view on the network.

## Mana Calculation

Mana is esentially the reputation score of a node in the IOTA network. Mana is calculated locally in each node, as a
function that takes value transactions as input and produces the Base Mana Vector as output.

A each transaction has an `accessMana` and `consensusMana` field that determine which node to pledge these two types
of mana to. Both of these fields denote a `nodeID`, the receiver of mana. `accessMana` and `consensusMana` do not have
to be pledged to the same node, but for simplicity, in the first implementation, they will be.

In addition to the mana fields, a `timestamp` field is also added to the transactions that will be utilized for calculating
decay and effective mana.

From the pledged mana of a transaction, a node can calculate locally the `Base Mana Vector` for both `Access Mana` and
`Consensus Mana`.

A `Base Mana Vector` consists of Base Mana 1 and Base Mana 2 and their respective `Effective Base Mana`.
Given a value transaction, Base Mana 1 and Base Mana 2 are determined as follows:
 1. Base Mana 1 is revoked from the node that created the output(s) used as input(s) in the transaction, and is pledged to
    the node creating the new output(s).
 2. Base Mana 2 is freshly created at the issuance time of the transaction, awarded to the node, but decays with time.

An example `Base Mana Vector` for `Access Mana` could look likt this:

 | 		   | Node 1 | Node 2 | ... | Node k
 |  ------- | --------- | ---------- | ------ | ---
 | Base Mana 1 			|0	| 1 	|...|  100.54
 | Effective Base Mana 1	|0	| 0.5 	|...|  120.7
 | Base Mana 2 			|0	| 1.2	|...|  0.01
 | Effective Base Mana 2	|0	| 0.6 	|...|  0.015

`Base Mana` is pledged or revoked at discrete times, which results in `Base Mana` being discontinuous function over time.
In order to make mana "smoother" and continuous, an exponential moving average is applied to the `Base Mana` values,
resulting in `Effective Base Mana 1` and `Effective Base Mana 2`.

It is important to note, that consuming a new transaction and pledging its mana happens when the transaction is
confirmed on the node. At the same time, entries of the nodes whose mana is being modified during pledging in the
`Base Mana Vector(s)` are updated with respect to time. In general, updates due to time happen whenever a node's mana is
being accessed. Except for the aforementioned case, this could be for example a mana related query from an external
module (FPC, Autopeering, DRNG, Rate Control, tools, etc.).

The exact mathematical formulas, and their respective parameters will be determined later.

## Mana Plugin in GoShimmer

Mana should be implemented as a separate plugin in GoShimmer.

The `mana plugin` is responsible for:
 - calculating mana from value transactions,
 - keeping a log of the different mana values of all nodes,
 - updating mana values,
 - responding to mana related queries from other modules.

 The proposed mana plugin should keep track of the different mana values of nodes and handle calculation
 updates. Mana values are mapped to `nodeID`s and stored in a `map` data structure, Note, that except for `Base Mana 1`,
 the time of the last update has to be stored together with the latest value:
 ```go
type BaseMana struct {
  BaseMana1 float
  EffectiveBaseMana1 float
  EBM1LastUpdated time
  BaseMana2 float
  BM2LastUpdated time
  EffectiveBaseMana2 float
  EBM2LastUpdated time
}

type BaseManaVector map[nodeID]BaseMana
```

`Access Mana` and `Consensus Mana` should have their own respective `BaseManaVector`. In the future, it should be possible
to combine `Effective Base Mana 1` and `Effective Base Mana 2` from a `BaseManaVector` in arbitrary proportions to arrive
at a final mana value that other modules use.

 The mana plugin should expose utility funtions to other modules for:
  1. Obtaining the list of N highest `Access`/`Consensus` mana nodes.
  2. Obtaining the `Access`/`Consensus` mana value of a particular node.
  3. Obtaining a list of currently known peers + their mana, sorted. Useful for knowing which high mana nodes are online.
  4. Obtaining a list of neighbors and their mana.
  5. Obtaining the full mana maps for comparison with the perception of other nodes.

  +1, consider adding new API routes for mana related infos.

 Such utility functions could be used for example to visualize mana distribution in node dashboard, or send neighbor
 mana data to the analysis server for further processing.

## Challenges

### Dependency on Value Tangle

Since mana is awarded to nodes submitting value transfers, the value tangle is needed as input for mana calculation.
Each node calculates mana locally, therefore, it is essential to determine when to consider transactions in the value
tangle "final enough" (so that they will not be orphaned).

When a transaction is `confirmed`, it is a sufficient indicator that it will not be orphaned. However, in current
GoShimmer iplementation, confirmation is not yet a properly defined concept. This issue will be addresses in a separate
module.

The Mana module assumes, that the value tangle's `TransactionConfirmed` event is the trigger condition to update the
mana state machine (base mana vectors).

### Value Transaction Layout

When processing value transactions in the value layer for mana calculation, either the original message containing the
value payload should be revisited to determine the correct `nodeID`, or a new field should be added to `Transaction`
denoting `PledgedNodeID` for `Access Mana` and `Consensus Mana`. The latter is also beneficial to implement mana donation
feature, that is, to donate the mana of a certain transaction ot an arbitrary node.

## Limitations

The first implementation of mana in goshimmer will:
  - not have voted timestamps on value transactions,
  - lack proper `TransactionConfirmed` mechanism to trigger mana update,
  - lack integration into rate control/autopeering/fpc/etc.
  
## Detailed Design

In this section, detailed GoShimmer implementation design considerations will be outlined about the mana module.
In short, changes can be classified into 3 categories:
 1. Value Tangle (Transaction) related changes,
 2. Mana module functionality,
 3. and related tools/utilities, such as API, visualization, analytics.

### Value Tangle

As described above, 3 new fields will be added to the transaction layout:
 1. `Timestamp` time.time
 2. `AccessManaNodeID` []bytes
 3. `ConsensusManaNodeID` []bytes

By adding these fields to the signed transaction, `valuetransfers/packages/transaction` should be modified.
 - The three new fields should be added to the transaction essence.
 - Marshalling and unmarshalling of a transaction should be modified.

**Open Question:**

`Timestamp` is part of the signed transaction, therefore, a client sending a transaction to the node should already
define it. In this case, this `Timestamp` will not be the same as the timestamp of the message containing the
transaction and value payload, since the message is created on the node.
A solution to this would be that upon receiving a `transaction` from a client, the node checks if the timestamp is not
older than a certain point in time (couple seconds ago?) and or is not in the future. Then constructs the message to have
the same timestamp. The message and the transaction should have the same timestamp because FPC will vote on this timestamp.

### Mana Module

The functionality of the mana module should be implemented in a `mana` package. Then, a `mana plugin` can use the package
structs and methods to connect the dots, for example execute `BookMana` when `TransactionConfirmed` event is triggered
in the value tangle.

The mana package should have the following events:
 - `ManaBooked` when mana was booked for a node due to new transactions being confirmed.
 - `ManaUpdated` when mana was updated for a node due to it being accessed/modified.

The mana package should expose the following methods:
 - `BookAccessMana(nodeID, amount)`: book access mana for a particular node. Trigger `ManaBooked` event.
 - `BookConsensusMana(nodeID, amount)`: book consensus mana for a particular node. Trigger `ManaBooked` event.
 - `GetAccessMana(nodeID) mana`: access `Base Mana Vector` of `Access Mana`, update its values with respect to time,
   and return the amount of `Access Mana` (either `Effective Base Mana 1`, `Effective Base Mana 2`, or some combination
   of the two). Trigger `ManaUpdated` event.
 - `GetConsensusMana(nodeID) mana`: access `Base Mana Vector` of `Consensus Mana`, update its values with respect to time,
   and returns the amount of `Consensus Mana` (either `Effective Base Mana 1`, `Effective Base Mana 2`, or some combination
   of the two). Trigger `ManaUpdated` event.
 - `GetHighestManaNodes(type, n) [n]NodeIdManaTuple`: return the `n` highest `type` mana nodes (`nodeID`,`manaValue`) in
   ascending order. Should also update their mana value.
 - `GetManaMap(type) map[nodeID]manaValue`: return `type` mana perception of the node.

 TO BE CONTINUED

#### Details of Mana Calculation
To be written.
Benchmark calculations in tests to see how heavy it is to calculate EMAs and decays.

### Mana Tools
TO BE WRITTEN
#### Mana Related API endpoints
#### Metrics collection
#### Visualization
