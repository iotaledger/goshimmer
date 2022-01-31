---
description: Mana is a reputation system for nodes within the IOTA network. Reputation is gained by contributing to the network. As time passes, part of the earned mana of a node decays to encourage keeping up the good behavior.
image: /img/protocol_specification/mana.png
keywords:
- mana
- node
- calculation
- transactions
- base mana
- vectors
- access mana
- consensus mana
- effective base mana
- ledger state
---
# Mana Implementation

This document provides a high level overview of how mana is implemented in GoShimmer.

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
    the node creating the new output(s). The amount of `Base Mana 1` revoked and pledged is equal to the balance of the
    input.
 2. Base Mana 2 is freshly created at the issuance time of the transaction, awarded to the node, but decays with time.
    The amount of `Base Mana 2` pledged is determined with `Pending Mana` concept: funds sitting at an address generate
    `pending mana` that grows over time, but bounded.
    - `Mana_pending = (alpha*S)/gamma*(1-e^(-gamma*t))`, where `alpha` and `gamma` are chosen parameters, `S` is the amount
      of funds an output transfers to the address, and `t` is the time since the funds are on that address.

An example `Base Mana Vector` for `Access Mana` could look like this:

 | 		    | Node 1 | Node 2 | ... | Node k |
 |  ------- | --------- | ---------- | ------ | --- |
 | Base Mana 1 			|0	| 1 	|...|  100.54 |
 | Effective Base Mana 1	|0	| 0.5 	|...|  120.7 |
 | Base Mana 2 			|0	| 1.2	|...|  0.01 |
 | Effective Base Mana 2	|0	| 0.6 	|...|  0.015 |

`Base Mana` is pledged or revoked at discrete times, which results in `Base Mana` being discontinuous function over time.
In order to make mana "smoother" and continuous, an exponential moving average is applied to the `Base Mana` values,
resulting in `Effective Base Mana 1` and `Effective Base Mana 2`.

It is important to note, that consuming a new transaction and pledging its mana happens when the transaction is
confirmed on the node. At the same time, entries of the nodes whose mana is being modified during pledging in the
`Base Mana Vector(s)` are updated with respect to time. In general, updates due to time happen whenever a node's mana is
being accessed. Except for the aforementioned case, this could be for example a mana related query from an external
module (AutoPeering, DRNG, Rate Control, tools, etc.).

Following figure summarizes how `Access Mana` and `Consensus Mana` is derived from a transaction:

[![Mana](/img/protocol_specification/mana.png "Mana")](/img/protocol_specification/mana.png)

The reason for having two separate `Base Mana Vectors` is the fact, that `accessMana` and `consensusMana` can be pledged
to different nodes.

The exact mathematical formulas, and their respective parameters will be determined later.

## Challenges

### Dependency on Tangle

Since mana is awarded to nodes submitting value transfers, the tangle is needed as input for mana calculation.
Each node calculates mana locally, therefore, it is essential to determine when to consider transactions in the
tangle "final enough" (so that they will not be orphaned).

When a transaction is `confirmed`, it is a sufficient indicator that it will not be orphaned. However, in current
GoShimmer implementation, confirmation is not yet a properly defined concept. This issue will be addressed in a separate
module.

The Mana module assumes, that the (value) tangle's `TransactionConfirmed` event is the trigger condition to update the
mana state machine (base mana vectors for access and consensus mana). Once the concept of transaction finality is
introduced for the tangle, the trigger conditions for access and consensus mana calculations can be adjusted.

### Transaction Layout

A new field should be added to `Transaction` denoting `PledgedNodeID` for `Access Mana` and `Consensus Mana`.
This is also beneficial to implement mana donation feature, that is, to donate the mana of a certain transaction to an
arbitrary node.

## Limitations

The first implementation of mana in GoShimmer will:
  - not have voted timestamps on value transactions,
  - lack proper `TransactionConfirmed` mechanism to trigger mana update,
  - lack integration into rate control/autopeering/etc.
  
## Detailed Design

In this section, detailed GoShimmer implementation design considerations will be outlined about the mana module.
In short, changes can be classified into 3 categories:
 1. Transaction related changes,
 2. Mana module functionality,
 3. and related tools/utilities, such as API, visualization, analytics.

### Transaction

As described above, 3 new fields will be added to the transaction layout:
 1. `Timestamp` time.time
 2. `AccessManaNodeID` []bytes
 3. `ConsensusManaNodeID` []bytes

By adding these fields to the signed transaction, `valuetransfers/packages/transaction` should be modified.
 - The three new fields should be added to the transaction essence.
 - Marshalling and unmarshalling of a transaction should be modified.
 - For calculating `Base Mana 1` values, `mana module` should be able to derive from a transaction the nodes which received
   pledged `Base Mana 1` as a consequence of the consumed inputs of the transaction. Therefore, a lookup function should
   be exposed from the value tangle that given an `input`, returns the `pledgedNodeID` of the transaction creating the input.

`Timestamp` is part of the signed transaction, therefore, a client sending a transaction to the node should already
define it. In this case, this `Timestamp` will not be the same as the timestamp of the message containing the
transaction and value payload, since the message is created on the node.
A solution to this is that upon receiving a `transaction` from a client, the node checks if the timestamp is within
a predefined time window, for example `t_current - delta`, where `delta` could be couple seconds. If true, then the node
constructs the message, which must have a greater timestamp, than the transaction.

`AccessManaNodeID` and `ConsensusManaNodeID` are also part of the signed transaction, so a client should fill them out.
Node owners are free to choose to whom they pledge mana to with the transaction, so there should be a mechanism that
lets the client know, what `AccessManaNodeID` and `ConsensusManaNodeID` are allowed. This could be a new API endpoint
that works like this:

 1. Client asks node what nodeIDs can be included for pledging  a certain type (access, consensus) mana.
 2. Node answers with either:
  - Don't care. Any node IDs are valid.
  - List of nodeIDs that are allowed for each type.
 3. If a client sends back the transaction with invalid or empty mana fields, the transaction is considered invalid.

This way node owners can decide who their transactions are pledging mana to. It could be only their node, or they could
provide mana pledging as a service. They could delegate access mana to others, but hold own to consensus mana, or the
other way around.

### Initialization

Mana state machine is an extension of the ledger state, hence its calculation depends on the ledger state perception
of the node. Snapshotting is the mechanism that saves the ledger states and prunes unnecessary transactions. Together
with the ledger state, base mana vectors are also saved, since a certain ledger state reflects a certain mana distribution
in the network. In future, when snapshotting is implemented in GoShimmer, nodes joining the network will be able to query
for snapshot files that will contain initial base mana vectors as well.

Until this functionality is implemented, mana calculation solely relies on transactions getting confirmed. That is, when
a node joins the network and starts gathering messages and transactions from peers, it builds its own ledger state through
solidification process. Essentially, the node requests all messages down to the genesis from the current tips of its neighbors.
Once the genesis is found, messages are solidified bottom up. For the value tangle, this means that for each solidified
and liked transaction, `TransactionConfirmed` event is triggered, updating the base mana vectors.

In case of a large database, initial synching and solidification is a computationally heavy task due to the sheer amount
of messages in the tangle. Mana calculation only adds to this burden. It will be determined through testing if additional
"weight lifting" mechanism is needed (for example delaying mana calculation).

In the GoShimmer test network, all funds are initially held by the faucet node, therefore all mana present at bootstrap belong
to this node. Whenever a transaction is requested from the faucet, it pledges mana to the requesting node, helping other
nodes to increase their mana.

### Mana Package

The functionality of the mana module should be implemented in a `mana` package. Then, a `mana plugin` can use the package
structs and methods to connect the dots, for example execute `BookMana` when `TransactionConfirmed` event is triggered
in the value tangle.

`BaseMana` is a struct that holds the different mana values for a given node.
Note that except for `Base Mana 1` calculation, we need the time when `BaseMana` values were updated, so we store it in the struct:
 ```go
type BaseMana struct {
  BaseMana1 float
  EffectiveBaseMana1 float
  BaseMana2 float
  EffectiveBaseMana2 float
  LastUpdated time
}
```

`BaseManaVector` is a data structure that maps `nodeID`s to `BaseMana`. It also has a `Type` that denotes the type
of mana this vector deals with (Access, Consensus, etc.).
```go
type BaseManaVector struct {
	vector     map[identity.ID]*BaseMana
	vectorType Type
}
```

#### Methods

`BaseManaVector` should have the following methods:
 - `BookMana(transaction)`: Book mana of a transaction. Trigger `ManaBooked` event. Note, that this method updates
   `BaseMana` with respect to time and to new `Base Mana 1` and `Base Mana 2` values.
 - `GetWeightedMana(nodeID, weight) mana`: Return `weight` *` Effective Base Mana 1` + (1-`weight`)+`Effective Base Mana 2`.
   `weight` is a number in [0,1] interval. Notice, that `weight` = 1  results in only returning `Effective Base Mana 1`,
   and the other way around. Note, that this method also updates `BaseMana` of the node with respect to time.
 - `GetMana(nodeID) mana`: Return 0.5*`Effective Base Mana 1` + 0.5*`Effective Base Mana 2` of a particular node. Note, that
   this method also updates `BaseMana` of the node with respect to time.
 - `update(nodeID, time)`: update `Base Mana 2`, `Effective Base Mana 1` and `Effective Base Mana 2` of a node with respect `time`.
 - `updateAll(time)`: update `Base Mana 2`, `Effective Base Mana 1` and `Effective Base Mana 2` of all nodes with respect to `time`.

`BaseMana` should have the following methods:
 - `pledgeAndUpdate(transaction)`: update `BaseMana` fields and pledge mana with respect to `transaction`.
 - `revokeBaseMana1(amount, time)`:  update `BaseMana` values with respect to `time` and revoke `amount` `BaseMana1`.
 - `update(time)`: update all `BaseMana` fields with respect to `time`.
 - `updateEBM1(time)`: update `Effective Base Mana 1` wrt to `time`.
 - `updateBM2(time)`: update `Base Mana 2` wrt to `time`.
 - `updateEBM2(time)`: update `Effective Base Mana 2` wrt to `time`.

#### Base Mana Calculation

There are two cases when the values within `Base Mana Vector` are updated:
 1. A confirmed transaction pledges mana.
 2. Any module accesses the `Base Mana Vector`, and hence its values are updated with respect to `access time`.

First, let's explore the former.

##### A confirmed transaction pledges mana

For simplicity, we only describe mana calculation for one of the Base Mana Vectors, namely, the Base Access Mana Vector.

First, a `TransactionConfirmed` event is triggered, therefore `BaseManaVector.BookMana(transaction)` is executed:
```go
func (bmv *BaseManaVector) BookMana(tx *transaction) {
    pledgedNodeID := tx.accessMana

    for input := range tx.inputs {
        // search for the nodeID that the input's tx pledged its mana to
        inputNodeID := loadPledgedNodeIDFromInput(input)
        // save it for proper event trigger
        oldMana := bmv[inputNodeID]
        // revoke BM1
        bmv[inputNodeID].revokeBaseMana1(input.balance, tx.timestamp)

        // trigger events
        Events.ManaRevoked.Trigger(&ManaRevokedEvent{inputNodeID, input.balance, tx.timestamp, AccessManaType})
        Events.ManaUpdated.Tigger(&ManaUpdatedEvent{inputNodeID, oldMana, bmv[inputNodeID], AccessManaType})
    }

    // save it for proper event trigger
    oldMana :=  bmv[pledgedNodeID]
    // actually pledge and update
    bm1Pledged, bm2Pledged := bmv[pledgedNodeID].pledgeAndUpdate(tx)

    // trigger events
    Events.ManaPledged.Trigger(&ManaPledgedEvent{pledgedNodeID, bm1Pledged, bm2Pledged, tx.timestamp, AccessManaType})
    Events.ManaUpdated.Trigger(&ManaUpdatedEvent{pledgedNodeID, oldMana, bmv[pledgedNodeID], AccessManaType})
}
```
`Base Mana 1` is being revoked from the nodes that pledged mana for inputs that the current transaction consumes.
Then, the appropriate node is located in `Base Mana Vector`, and mana is pledged to its `BaseMana`.
`Events` are essential to study what happens within the module from the outside.

Note, that `revokeBaseMana1` accesses the mana entry of the nodes within `Base Mana Vector`, therefore all values are
updated with respect to `t`. Notice the two branches after the condition. When `Base Mana` values had been updated before
the transaction's timestamp, a regular update is carried out. However, if `t` is older, than the transaction timestamp,
an update in the "past" is carried out and values are updated up to `LastUpdated`.
```go
func (bm *BaseMana) revokeBaseMana1(amount float64, t time.Time) {
	if t.After(bm.LastUpdated) {
		// regular update
		n := t.Sub(bm.LastUpdated)
		// first, update EBM1, BM2 and EBM2 until `t`
		bm.updateEBM1(n)
		bm.updateBM2(n)
		bm.updateEBM2(n)

		bm.LastUpdated = t
		// revoke BM1 at `t`
		bm.BaseMana1 -= amount
	} else {
		// update in past
		n := bm.LastUpdated.Sub(t)
		// revoke BM1 at `t`
		bm.BaseMana1 -= amount
		// update EBM1 to `bm.LastUpdated`
		bm.EffectiveBaseMana1 -= amount*(1-math.Pow(math.E,-EMA_coeff_1*n))
	}
}
```
The same regular and past update scheme is applied to pledging mana too:
```go
func (bm *BaseMana) pledgeAndUpdate(tx *transaction) (bm1Pledged int, bm2Pledged int){
	t := tx.timestamp
	bm1Pledged = sum_balance(tx.inputs)

	if t.After(bm.LastUpdated) {
		// regular update
		n := t.Sub(bm.LastUpdated)
		// first, update EBM1, BM2 and EBM2 until `t`
		bm.updateEBM1(n)
		bm.updateBM2(n)
		bm.updateEBM2(n)
		bm.LastUpdated = t
		bm.BaseMana1 += bm1Pledged
		// pending mana awarded, need to see how long funds sat
		for input := range tx.inputs {
			// search for the timestamp of the UTXO that generated the input
			t_inp := LoadTxTimestampFromOutputID(input)
			bm2Add := input.balance * (1 - math.Pow(math.E, -decay*(t-t_inp)))
			bm.BaseMana2 += bm2Add
			bm2Pledged += bm2Add
		}
	} else {
		// past update
		n := bm.LastUpdated.Sub(t)
		// update BM1 and BM2 at `t`
		bm.BaseMana1 += bm1Pledged
		oldMana2 = bm.BaseMana2
		for input := range tx.inputs {
			// search for the timestamp of the UTXO that generated the input
			t_inp := LoadTxTimestampFromOutputID(input)
			bm2Add := input.balance * (1-math.Pow( math.E,-decay*(t-t_inp) ) ) * math.Pow(math.E, -decay*n)
			bm.BaseMana2 += bm2Add
			bm2Pledged += bm2Add
		}
		// update EBM1 and EBM2 to `bm.LastUpdated`
		bm.EffectiveBaseMana1 += amount*(1-math.Pow(math.E,-EMA_coeff_1*n))
		if EMA_coeff_2 != decay {
			bm.EffectiveBaseMana2 += (bm.BaseMana2 - oldMana2) *EMA_coeff_2*(math.Pow(math.E,-decay*n)-
                math.Pow(math.E,-EMA_coeff_2*n))/(EMA_coeff_2-decay) / math.Pow(math.E, -decay*n)
		} else {
			bm.EffectiveBaseMana2 += (bm.BaseMana2 - oldMana2) * decay * n
		}
}
	return bm1Pledged, bm2Pledged
}
```
Notice, that in case of `EMA_coeff_2 = decay`, a simplified formula can be used to calculate `EffectiveBaseMana2`.
The same approach is applied in `updateEBM2()`.

```go
func (bm *BaseMana) updateEBM1(n time.Duration) {
    bm.EffectiveBaseMana1 = math.Pow(math.E, -EMA_coeff_1 * n) * bm.EffectiveBaseMana1 +
                                 (1-math.Pow(math.E, -EMA_coeff_1 * n)) * bm.BaseMana1
}
```
```go
func (bm *BaseMana) updateBM2(n time.Duration) {
    bm.BaseMana2 = bm.BaseMana2 * math.Pow(math.E, -decay*n)
}
```
```go
func (bm *BaseMana) updateEBM2(n time.Duration) {
	if EMA_coeff_2 != decay {
		bm.EffectiveBaseMana2 = math.Pow(math.E, -emaCoeff2 * n) * bm.EffectiveBaseMana2 +
			(math.Pow(math.E, -decay * n) - math.Pow(math.E, -EMA_coeff_2 * n)) /
				(EMA_coeff_2 - decay) * EMA_coeff_2 / math.Pow(math.E, -decay * n)*bm.BaseMana2
	} else {
		bm.EffectiveBaseMana2 = math.Pow(math.E, -decay * n)*bm.EffectiveBaseMana2 +
			decay * n * bm.BaseMana2
	}
}
```

##### Any module accesses the Base Mana Vector

In this case, the accessed entries within `Base Mana Vector` are updated via the method:
```go
func (bmv *BaseManaVector) update(nodeID ID, t time.Time ) {
    oldMana :=  bmv[nodeID]
    bmv[nodeID].update(t)
    Events.ManaUpdated.Trigger(&ManaUpdatedEvent{nodeID, oldMana, bmv[nodeID], AccessManaType})
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

 - `Pledged` when mana (`BM1` and `BM2`) was pledged for a node due to new transactions being confirmed.

```go
type PledgedEvent struct {
    NodeID []bytes
    AmountBM1 int
    AmountBM2 int
    Time time.Time
    Type ManaType // access or consensus
}
```

- `Revoked` when mana (`BM1`) was revoked from a node.

```go
type RevokedEvent struct {
    NodeID []bytes
    AmountBM1 int
    Time time.Time
    Type ManaType // access or consensus
}
```

 - `Updated` when mana was updated for a node due to it being accessed.

```go
type UpdatedEvent struct {
    NodeID []bytes
    OldMana BaseMana
    NewMana BaseMana
    Type    ManaType // access or consensus
}
```

#### Testing
 
 - Write unit tests for all methods.
 - Test all events and if they are correctly triggered.
 - Benchmark calculations in tests to see how heavy it is to calculate EMAs and decays.

### Mana Plugin

The `mana plugin` is responsible for:
 - calculating mana from value transactions,
 - keeping a log of the different mana values of all nodes,
 - updating mana values,
 - responding to mana related queries from other modules,
 - saving base mana vectors in database when shutting down the node,
 - trying to load base mana vectors from database when starting the node.

The proposed mana plugin should keep track of the different mana values of nodes and handle calculation
updates. Mana values are mapped to `nodeID`s and stored in a `map` data structure. The vector also stores information on
what `Type` of mana it handles.

```go
type BaseManaVector struct {
	vector     map[identity.ID]*BaseMana
	vectorType Type
}
```

`Access Mana` and `Consensus Mana` should have their own respective `BaseManaVector`.
```go
accessManaVector := BaseManaVector{vectorType: AccesMana}
consensusManaVector :=  BaseManaVector{vectorType: ConsensusMana}
```
In the future, it should be possible to combine `Effective Base Mana 1` and `Effective Base Mana 2` from a `BaseManaVector`
in arbitrary proportions to arrive at a final mana value that other modules use. The `mana package` has these methods
in place. Additionally, a parameter could be passed to the `getMana` type of exposed functions to set the proportions.

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

#### Synchronization and Mana Calculation

The mana plugin is responsible to determine when to start calculating mana locally.
Since mana state is an extension to ledger state, it can only depict realistic mana values once the node is in sync.
During syncing, ledger state is constructed from messages coming from neighbors as described further above.

In this first iteration, mana plugin relies on `TransactionConfirmed` event of the value transfers plugin, and has no
explicit rules on when to start and stop mana calculation.

In future, initial mana state (together with the initial ledger state) will be derived from a snapshot file.

### Mana Toolkit

In this section, all tools and utility functions for mana will be outlined.

#### Mana Related API endpoints

 - `/info`: Add own mana in node info response.
 - `value/allowedManaPledge`: Endpoint that clients can query to determine which nodeIDs are allowed as part of
   `accessMana` and `consensusMana` fields in a transaction.
 - `value/sendTransactionByJson`: Add `accessMana`, `consensusMana` and `timestamp` fields to the JSON request.

Add a new `mana` endpoint route:
 - `/mana`: Return access and consensus mana of the node.
 - `/mana/all`: Return whole mana map (mana perception of the node).
 - `/mana/access/nhighest`: Return `n` highest access mana holder `nodeIDs` and their access mana values.
 - `/mana/consensus/nhighest`: Return `n` highest consensus mana holder `nodeIDs` and their consensus mana values.
 - `/mana/percentile`: Return the top percentile the node belongs to relative to the network. For example, if there are 100 nodes in the
   network owning mana, and a node is the 13th richest, it means that is part of the top 13% of mana holders, but not the
   top 12%.

#### Metrics collection

To study the mana module, following metrics could be gathered:

 - Amount of consensus and access mana present in the network. (amount varies because of `Base Mana 2`).
 - Amount of mana each node holds.
 - Number of (and amount of mana) a node was pledged with mana in the last `t` interval.
 - Mana development of a particular node over time.
 - Mana percentile development of a node over time.
 - Average pledge amount of a node. (how much mana it receives on average with one pledge)
 - Mean and median mana holdings of nodes in the network. Shows how even mana distribution is.
 - Average mana of neighbors.

#### Visualization

Each node calculates mana locally, not only for themselves, but for all nodes in the network that it knows. As a result,
mana perception of nodes may not be exactly the same at all times (due to network delay, processing capabilities), but
should converge to the same state. A big question for visualization is which node's viewpoint to base mana visualization on? 

When running a node, operators will be shown the mana perception of their own node, but it also makes sense to
display the perception of high mana nodes as the global mana perception. First, let's look at how local mana perception
is visualized for a node:

##### Local Perception

There are two ways to visualize mana in GoShimmer:
 1. Node Local Dashboard
 2. Grafana Dashboard

While `Local Dashboard` gives flexibility in what and how to visualize, `Grafana Dashboard` is better at storing historic
data but can only visualize time series. Therefore, both of these ways will be utilized, depending on which suits the best.

`Local Dashboard` visualization:
 - Histogram of mana distribution within the network.
 - List of `n` richest mana nodes, ordered.
 - Mana rank of node.

`Grafana Dashboard` visualization:
 - Mana of a particular node with respect to time.
 - Amount of mana in the network.
 - Average pledge amount of a node.
 - Mean and median mana holdings of nodes.
 - Mana rank of the node over time.
 - Average mana of neighbors.

 ##### Global Perception

Additionally, the GoShimmer Analyzer (analysis server) could be updated:
 - Autopeering node graph, where size of a node corresponds to its mana value.
 - Some previously described metrics could be visualized here as well, to give the chance to people without
   a node to take a look. As an input, a high mana node's perception should be used.
