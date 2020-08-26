# Mana Implementation Proposal

The goal of this document is to provide a high level overview of how mana will be implemented in GoShimmer.

## Introduction

Mana is a reputation system for nodes within the IOTA network.

Reputation is gained by contributing to the network, i.e. creating value transfers.
As time passes, part of the earned mana of a node decays to encourage keeping up the good behavior.

## Scope

The scope of the first implementation of mana into GoShimmer is to verify that mana calculations work,
study base mana calculations 1 & 2, and mana distribution in the test network, furthermore to verify that nodes have
similar view on the network.

## Mana Calculation

Mana is essentially the reputation score of a node in the IOTA network. Mana is calculated locally in each node, as a
function that takes value transactions as input and produces the Base Mana Vector as output.

Each transaction has an `accessMana` and `consensusMana` field that determine which node to pledge these two types
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

 | 		    | Node 1 | Node 2 | ... | Node k
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

Following figure summarizes how `Access Mana` and `Consensus Mana` is derived from a transaction:

![](https://i.imgur.com/LjfCTm9.png)

The reason for having two separate `Base Mana Vectors` is the fact, that `accessMana` and `consensusMana` can be pledged to different nodes

The exact mathematical formulas, and their respective parameters will be determined later.

## Challenges

### Dependency on Value Tangle

Since mana is awarded to nodes submitting value transfers, the value tangle is needed as input for mana calculation.
Each node calculates mana locally, therefore, it is essential to determine when to consider transactions in the value
tangle "final enough" (so that they will not be orphaned).

When a transaction is `confirmed`, it is a sufficient indicator that it will not be orphaned. However, in current
GoShimmer implementation, confirmation is not yet a properly defined concept. This issue will be addresses in a separate
module.

The Mana module assumes, that the value tangle's `TransactionConfirmed` event is the trigger condition to update the
mana state machine (base mana vectors).

### Value Transaction Layout

When processing value transactions in the value layer for mana calculation, either the original message containing the
value payload should be revisited to determine the correct `nodeID`, or a new field should be added to `Transaction`
denoting `PledgedNodeID` for `Access Mana` and `Consensus Mana`. The latter is also beneficial to implement mana donation
feature, that is, to donate the mana of a certain transaction ot an arbitrary node.

## Limitations

The first implementation of mana in GoShimmer will:
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

### Initialization

- When a node starts, it will query other nodes to get their baseManaVectors that will be used for initialization.
- When the faucet starts if queries baseManVectors from other nodes as above. If its own `nodeID` is not found, it sets
  its mana to be equal to its available funds. Faucets mana will be pledged to the node that requests for funds.

**Open Question**
- How many nodes to query? All neighbors?
- What is the selection algorithm? Random?
- If the facuet restarts

### Mana Package

The functionality of the mana module should be implemented in a `mana` package. Then, a `mana plugin` can use the package
structs and methods to connect the dots, for example execute `BookMana` when `TransactionConfirmed` event is triggered
in the value tangle.

`BaseMana` is a struct that holds the different mana values for a given node.
Note that except for `Base Mana 1` calculation, we need the time the `BaseMana` values were updated, so we store it in the struct:
 ```go
type BaseMana struct {
  BaseMana1 float
  EffectiveBaseMana1 float
  BaseMana2 float
  EffectiveBaseMana2 float
  LastUpdated time
}
```

`BaseManaVector` is a data structure that maps `nodeID`s to `BaseMana`.
```go
type BaseManaVector map[nodeID]BaseMana
```

#### Methods
`BaseManaVector` should have the following methods:
 - `BookMana(transaction)`: Book mana of a transaction. Trigger `ManaBooked` event. Note, that this method updates
   `BaseMana` with respect to time and to new `Base Mana 1` and `Base Mana 2` values.
 - `GetTotalMana(nodeID) mana`: Return `Effective Base Mana 1` + `Effective Base Mana 2` of a particular node. Note, that
   this method also updates `BaseMana` of the node with respect to time.
 - `GetMana(nodeID, weightEBM1, weightEBM2) mana`: Return `weightEBM1` *` Effective Base Mana 1` + `weightEBM1`+`Effective Base Mana 2`.
   `weight` is a number in [0,1] interval. Notice, that `weightEBM1` = 1 and `weightEBM2`= 0 results in only returning `Effective Base Mana 1`,
   and also the other way around. Note, that this method also updates `BaseMana` of the node with respect to time.
 - `update(nodeID, time)`: update `Base Mana 2`, `Effective Base Mana 1` and `Effective Base Mana 2` of a node with respect `time`.
 - `updateAll(time)`: update `Base Mana 2`, `Effective Base Mana 1` and `Effective Base Mana 2` of all nodes with respect to `time`.

`BaseMana` should have the following methods:
 - `pledgeAndUpdate(amount, time)`: pledge `amount` mana, and update `BaseMana` fields.
 - `revokeBaseMana1(amount, time)`: revoke `amount` `BaseMana1` and update `Effective Base Mana 1` with respect to `time.`
 - `update(time)`: update all `BaseMana` fields with respect to `time`.

#### Base Mana Calculation

There are two cases when the values within `Base Mana Vector` are updated:
 1. A confirmed transaction pledges mana.
 2. Any module accesses the `Base Mana Vector`, and hence its values are updated with respect to the `access time`.

First, let's explore the former.

##### A confirmed transaction pledges mana
For simplicity, we only describe mana calculation for one of the Base Mana Vectors, namely, the Base Access Mana Vector.

First, a `TransactionConfirmed` event is triggered, therefore `BaseManaVector.BookMana(transaction)` is executed:
```go
func (bmv *BaseManaVector) BookMana(tx transaction) {
    t := tx.timestamp
    pledgedNodeID := tx.accessMana

    amount := sum_balances(tx.outputs)

    for input := range tx.inputs {
        // search for the nodeID that the input's tx pledged its mana to
        inputNodeID := loadPledgedNodeIDFromInput(input)
        bmv[inputNodeID].revokeBaseMana1(input.balance, t)
    }

    bmv[pledgedNodeID].pledgeAndUpdate(amount, time)
}
```
`Base Mana 1` is being revoked from the nodes that pledged mana for the inputs that the current transaction consumes.
Then the appropriate node is located in `Base Mana Vector`, and sum of the transaction outputs amount of mana is pledged to it's `BaseMana`.

Note, that `revokeBaseMana1` accesses the mana entry of the nodes within `Base Mana Vector`, therefore all values are updated regarding to `t`.
```go
func (bm *BaseMana) revokeBaseMana1(amount float64, t time.Time) {
    n := t - bm.LastUpdated
    // first, update EBM1, BM2 and EBM2 until `t`
    bm.updateEBM1(n)
    bm.updateBM2(n)
    bm.updateEBM2(n)

    bm.LastUpdated = t
    // revoke BM2 at `t`
    bm.BaseMana2 -= amount
}
```

```go
func (bm *BaseMana) pledgeAndUpdate(amount float64, t time.Time) {
    n := t - bm.LastUpdated
    // first, update EBM1, BM2 and EBM2 until `t`
    bm.updateEBM1(n)
    bm.updateBM2(n)
    bm.updateEBM2(n)

    bm.LastUpdated = t

    bm.BaseMana1 += amount
    bm.BaseMana2 += amount
}
```

```go
func (bm *BaseMana) updateEBM1(n time.Duration) {
    bm.EffectiveBaseMana1 = math.Pow(1-EMA_coeff, n) * bm.EffectiveBaseMana1 +
                                 (1-math.Pow(1-EMA_coeff, n)) * bm.BaseMana1
}
```
```go
func (bm *BaseMana) updateBM2(n time.Duration) {
    bm.BaseMana2 = bm.BaseMana2 * math.Pow(math.E, -decay*n)
}
```
```go
func (bm *BaseMana) updateEBM2(n time.Duration) {
    bm.EffectiveBaseMana2 = math.Pow(1-EMA_coeff, n) * bm.EffectiveBaseMana2 +
                                (1-math.Pow(1-EMA_coeff, n)) * math.Pow(math.E, decay*n) /
                                (1-(1-EMA_coeff)*math.Pow(math.E, decay)) * EMA_coeff * bm.BaseMana2
}
```

##### Any module accesses the Base Mana Vector

In this case, the accessed entries within `Base Mana Vector` are updated via the method:
```go
func (bmv *BaseManaVector) update(nodeID ID, t time.Time ) {
    bmv[nodeID].update(t)
}
```
where `t` is the access time.
```go
func (bm *BaseMana) update(t time.Time ) {
    n := t - bm.LastUpdated
    bm.updateEBM1(n)
    bm.updateBM2(n)
    bm.updateEBM2(n)

    bm.LastUpdated = t
}
```

#### Events
The mana package should have the following events:
 - `ManaBooked` when mana was booked for a node due to new transactions being confirmed.
 ```go
type ManaBookedEvent struct {
    NodeID []bytes
    Mana BaseMana
}
```
 - `ManaUpdated` when mana was updated for a node due to it being accessed/modified.
 ```go
type ManaUpdatedEvent struct {
    NodeID []bytes
    OldMana BaseMana
    NewMana BaseMana
    Type    ManaType
}
```

### Mana Plugin

The `mana plugin` is responsible for:
 - calculating mana from value transactions,
 - keeping a log of the different mana values of all nodes,
 - updating mana values,
 - responding to mana related queries from other modules.

The proposed mana plugin should keep track of the different mana values of nodes and handle calculation
updates. Mana values are mapped to `nodeID`s and stored in a `map` data structure.

```go
type BaseManaVector map[nodeID]BaseMana
```

`Access Mana` and `Consensus Mana` should have their own respective `BaseManaVector`.
```go
accessManaVector BaseManaVector
consensusManaVector BaseManaVector
```
In the future, it should be possible to combine `Effective Base Mana 1` and `Effective Base Mana 2` from a `BaseManaVector`
in arbitrary proportions to arrive at a final mana value that other modules use.

#### Methods
The mana plugin should expose utility functions to other modules:
 - `GetHighestManaNodes(type, n) [n]NodeIdManaTuple`: return the `n` highest `type` mana nodes (`nodeID`,`manaValue`) in
   ascending order. Should also update their mana value.
 - `GetManaMap(type) map[nodeID]manaValue`: return `type` mana perception of the node.
 - `GetAccessMana(nodeID) mana`: access `Base Mana Vector` of `Access Mana`, update its values with respect to time,
    and return the amount of `Access Mana` (either `Effective Base Mana 1`, `Effective Base Mana 2`, or some combination
    of the two). Trigger `ManaUpdated` event.
 - `GetConsensusMana(nodeID) mana`: access `Base Mana Vector` of `Consensus Mana`, update its values with respect to time,
    and returns the amount of `Consensus Mana` (either `Effective Base Mana 1`, `Effective Base Mana 2`, or some combination
    of the two). Trigger `ManaUpdated` event.
 - `GetNeighborsMana(type)`: returns the `type` mana of the nodes neighbors
 - `GetAllManaVectors()` Obtaining the full mana maps for comparison with the perception of other nodes.
 - `GetWeightedRandomNodes(n)`: returns a weighted random selection of `n` nodes. `Consensus Mana` is used for the weights.
 - Obtaining a list of currently known peers + their mana, sorted. Useful for knowing which high mana nodes are online.
 - `OverrideMana(nodeID, baseManaVector)`: Sets the nodes mana to a specific value. Can be useful for debugging, setting faucet mana, initialization, etc.. Triggers `ManaUpdated`

Such utility functions could be used for example to visualize mana distribution in node dashboard, or send neighbor
mana data to the analysis server for further processing.

#### Booking Mana
Mana is booked when a transaction is confirmed.
 ```go
on TransactionConfirmed (tx):
  bookAccessMana()
  bookConsensusMana()
```

#### Details of Mana Calculation
To be written.
Benchmark calculations in tests to see how heavy it is to calculate EMAs and decays.

### Mana Tools
TO BE WRITTEN
### Mana Related API endpoints
 - `/info`: Add mana in node info
 - `/sendtransaction`: Add `nodeId` to pledge mana to

### Metrics collection
TO BE WRITTEN

### Visualization
We maintain chart data `[]<nodeID,mana>` for each node.
Initially, we have a certain mana at time `t0` for every node. When `ManaUpdated` event is triggered for a node, we get the new mana at time `t1` and can plot the changes for that node.
 - Mana distribution over the network. A pie chart of `<nodeID:value>`
 - Mana of a node wrt time. A LineGraph. The node plugs into `ManaUpdated` event.
 Question: Should this be local to the nodes dashboard or public? i.e Any node can visualize mana charts for another node via the public analyzer?
