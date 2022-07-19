---
description: High-level description of the interaction between components of the currently implemented GoShimmer protocol. The protocol can be divided into three main elements. A P2P overlay network, an immutable data structure, and a consensus mechanism.
image: /img/protocol_specification/layers.png
keywords:
- network layer
- node
- block
- ledger state
- data flow
- past cone
- future cone
- timestamp
- opinion setting
- strong tip
- tip pool
---

# Components of the Protocol

This section provides a high-level description of the interaction between components of the currently implemented GoShimmer protocol. The protocol can be divided into three main elements: a P2P overlay network, an immutable data structure, and a consensus mechanism. We abstract these three elements into layers, where—similarly to other architectures—upper layers build on the functionality provided by the layers below them. The definition of the different layers is merely about the convenience of creating a clear separation of concerns.

[![Components of the Protocol](/img/protocol_specification/layers.png "Components of the Protocol")](/img/protocol_specification/layers.png)

## Network Layer

The network is maintained by the network layer modules, which can be characterized as a pure P2P overlay network, meaning that it is a system that runs on top of another network (e.g., the internet), and where all nodes have the same roles and perform the same actions (in contrast to client-server systems). GoShimmer's Network Layer consists of three basic modules: the [peer discovery](autopeering.md#peer-discovery) module (which provides a list of nodes actively using the network), and the [neighbor selection](autopeering.md#neighbor-selection) module (also known as autopeering), which actually selects peers. Finally, the P2P Communication manages a node's neighbors, either selected via [autopeering](autopeering.md) or [manual peering](../../tutorials/manual_peering.md).

## Communication Layer

The communication layer concerns the information propagated through the network layer, which is contained in objects called blocks. This layer forms a DAG with blocks as vertices called the [Tangle](tangle.md): a replicated, shared and distributed data structure that emerges—through a combination of deterministic rules, cooperation, and virtual voting.
Since nodes have finite capabilities, the number of blocks that the network can process is limited. Thus, the network might become overloaded, either simply because of honest heavy usage or because of malicious (spam) attacks. To protect the network from halting or even from getting inconsistent, the rate control (currently a static PoW) and [congestion control](congestion_control.md) modules control when and how many blocks can be gossiped.

## (Decentralized) Application Layer

On top of the communication layer lives the application layer. Anybody can develop applications that run on this layer, and nodes can choose which applications to run. Of course, these applications can also be dependent on each other.
There are several core applications that must be run by all nodes, as the value transfer applications, which maintains the [ledger state](ledgerstate.md) (including  advanced [output types](advanced_outputs.md)), and a quantity called [Mana](mana.md), that serves as a scarce resource as our Sybil protection mechanism.
Additionally, all nodes must run what we call the consensus applications, which regulate timestamps in the blocks and resolve conflicts.
The consensus mechanism implemented in GoShimmer is leaderless and consists out of multiple components:
1. [Approval Weight](consensus_mechanism.md#approval-weight-aw) is an objective measure to determine the grade of finality of blocks and conflicts based on [active cMana](consensus_mechanism.md#Active-cMana).
2. The [Modular Conflict Selection Function](consensus_mechanism.md#modular-conflict-selection-function) is an abstraction on how a node sets an initial opinion on conflicts based on the .

## Data Flow - Overview

The diagram below represents the interaction between the different modules in the protocol ([event driven](../../implementation_design/event_driven_model.md)). Each blue box represents a component of the [Tangle codebase](https://github.com/iotaledger/goshimmer/tree/develop/packages/tangle), which has events (in yellow boxes) that belong to it. Those events will trigger methods (the green boxes), that can also trigger other methods. This triggering is represented by the arrows in the diagram. Finally, the purple boxes represent events that do not belong to the component that triggered them.

As an example, take the Parser component. The function `ProcessGossipBlock` will trigger the method `Parse`, which is the only entry to the component. There are three possible outcomes to the `Parser`: triggering a `ParsingFailed` event, a `BlockRejected` event, or a `BlockParsed` event. In the last case, the event will trigger the `StoreBlock` method (which is the entry to the Storage component), whereas the first two events do not trigger any other component.

[![Data Flow - Overview](/img/protocol_specification/data-flow.png "Data Flow - Overview")](/img/protocol_specification/data-flow.png)

We call this the data flow, i.e., the [life cycle of a block](../protocol.md), from block reception (meaning that we focus here on the point of view of a node receiving a block issued by another node) up until acceptance in the Tangle. Notice that any block, either created locally by the node or received from a neighbor needs to pass through the data flow.

### Block Factory

The IssuePayload function creates a valid payload which is provided to the `CreateBlock` method, along with a set of parents chosen with the Tip Selection Algorithm. Then, the Block Factory component is responsible to find a nonce compatible with the PoW requirements defined by the rate control module. Finally, the block is signed. Notice that the block generation should follow the rates imposed by the rate setter, as defined in [rate setting](congestion_control.md#rate-setting).

### Parser

The first step after the arrival of the block to the block inbox is the parsing, which consists of the following different filtering processes (meaning that the blocks that don't pass these steps will not be stored):

**Bytes filter**:
1. Recently Seen Bytes: it compares the incoming blocks with a pool of recently seen bytes to filter duplicates.
2. PoW check: it checks if the PoW requirements are met, currently set to the block hash starting with 22 zeroes.

Followed by the bytes filters, the received bytes are parsed into a block and its corresponding payload and [syntactically validated](tangle.md#syntactical-validation). From now on, the filters operate on block objects rather than just bytes.

**Block filter**:
1. Signature check: it checks if the block signature is valid.
2. [Timestamp Difference Check for transactions](tangle.md#block-timestamp-vs-transaction-timestamp): it checks if the timestamps of the payload, and the block are consistent with each other


### Storage

Only blocks that pass the Parser are stored, along with their metadata. Additionally, new blocks are stored as children of their parents, i.e., a reverse mapping that enables us to walk the Tangle into the future cone of a block.

### Solidifier

[Solidification](tangle.md#Solidification) is the process of requesting missing blocks. In this step, the node checks if all the past cone of the block is known; in the case that the node realizes that a block in the past cone is missing, it sends a request to its neighbors asking for that missing block. This process is recursively repeated until all of a block's past cone up to the genesis (or snapshot) becomes known to the node. 
This way, the protocol enables any node to retrieve the entire block history, even for nodes that have just joined the network.

### Scheduler

The scheduler makes sure that the network as a whole can operate with maximum throughput and minimum delays while providing consistency, fairness (according to aMana), and security. It, therefore, regulates the allowed influx of blocks to the network as a [congestion-control mechanism](congestion_control.md).


### Booker

After scheduling, the block goes to the booker. This step is different between blocks that contain a transaction payload and blocks that do not contain it.

In the case of a non-transaction payload, booking into the Tangle occurs after the conflicting parents conflicts check, i.e., after checking if the parents' conflicts contain sets of (two or more) transactions that belong to the same conflict set. In the case of this check not being successful, the block is marked as `invalid` and not booked.

In the case of a transaction as payload, initially, the following check is done:

1. UTXO check: it checks if the inputs of the transaction were already booked. If the block does not pass this check, the block is not booked. If it passes the check, it goes to the next block of steps.
2. Balances check: it checks if the sum of the values of the generated outputs equals the sum of the values of the consumed inputs. If the block does not pass this check, the block is marked as `invalid` and not booked. If it passes the check, it goes to the next step.
3. Unlock conditions: checks if the unlock conditions are valid. If the block does not pass this check, the block is marked as `invalid` and not booked. If it passes the check, it goes to the next step.
4. Inputs' conflicts validity check: it checks if all the consumed inputs belong to a valid conflict. If the block does not pass this check, the block is marked as `invalid` and not booked. If it passes the check, it goes to the next step.

After the objective checks, the following subjective checks are done:

5. Inputs' conflicts rejection check: it checks if all the consumed inputs belong to a non-rejected conflict. Notice that this is not an objective check, so the node is susceptible (even if with a small probability) to have its opinion about rejected conflicts changed by a reorganization. For that reason, if the block does not pass this check, the block is booked into the Tangle and ledger state (even though the balances are not altered by this block, since it will be booked to a rejected conflict). This is what we call "lazy booking", which is done to avoid huge re-calculations in case of a reorganization of the ledger. If it passes the check, it goes to the next step.
10. Double spend check: it checks if any of the inputs is conflicting with a transaction that was already confirmed. As in the last step, this check is not objective and, thus, if the block does not pass this check, it is lazy booked into the Tangle and ledger state, into an invalid conflict. If it passes the check, it goes to the next step.

At this point, the missing steps are the most computationally expensive:

7.  Inputs' conflicting conflicts check: it checks if the conflicts of the inputs are conflicting. As in the last step, if the block does not pass this check, the block is marked as `invalid` and not booked. If it passes the check, it goes to the next step.
8. Conflict check: it checks if the inputs are conflicting with an unconfirmed transaction. In this step, the conflict to which the block belongs is computed. In both cases (passing the check or not), the transaction is booked into the ledger state and the block is booked into the Tangle, but its conflict ID will be different depending on the outcome of the check.

[![Booker](/img/protocol_specification/booker.png "Booker")](/img/protocol_specification/booker.png)

Finally, after a block is booked, it might become a [marker](markers.md) (depending on the marker policy) and can be gossiped.

### Consensus Mechanism
A detailed description can be found [here](consensus_mechanism.md).


### Tip Manager

The first check done in the tip manager is the eligibility check (i.e., subjective timestamp is ok), after passing it, a block is said to be `eligible` for tip selection (otherwise, it's `not eligible`). 
If a block is eligible for [tip selection](tangle.md#tsa) and its payload is `liked`, along with all its weak past cone, the block is added to the strong tip pool and its parents are removed from the strong tip pool. If a block is eligible for tip selection, its payload is `liked` but its conflict is not liked it is added to the weak tip pool.
