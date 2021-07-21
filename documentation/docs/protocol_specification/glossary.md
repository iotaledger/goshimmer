# Glossary

## Consensus

Agreement on a specific datum or value in distributed multi-agent systems, in the presence of faulty processes.

### Blockchain Bottleneck

As more [transactions](#transaction) are issued, the block rate and size become a bottleneck in the system. It can no longer include all incoming transactions promptly. Attempts to speed up block rates will introduce more [orphan](#orphan) blocks (blocks being left behind) and reduce the security of the blockchain.

### Nakamoto Consensus

Named after the originator of Bitcoin, Satoshi Nakamoto, Nakamoto consensus describes the replacement of voting/communication between known agents with a cryptographic puzzle (Proof-of-Work). Completing the puzzle determines which agent is the next to act.

### Mining Races

In PoW-based DLTs, competition between [nodes](#node) to obtain mining rewards and [transactions](#transaction) fees are known as mining races. These are undesirable as they favor more powerful nodes, especially those with highly optimized hardware like ASICs. As such, they block participation by regular or IoT hardware and are harmful for the environment.

### Proof of work

Data which is difficult (costly, time-consuming) to produce but easy for others to verify.

## Coordinator

A trusted entity that issues milestones to guarantee [finality](#finality) and protect the [Tangle](#tangle) against attacks.

### Milestones

Milestones are [transactions](#transaction) signed and issued by the [Coordinator](#coordinator). Their main goal is to help the Tangle to grow healthily and to guarantee [finality](#finality). When milestones directly or indirectly approve a transaction in the [Tangle](#tangle), [nodes](#node) mark the state of that transaction and its entire [history](#history) as confirmed.

## Tangle

An append-only [message](#message) data structure where each message references (at least) two other messages.

### Sub-tangle

A consistent section of the [Tangle](#tangle) (a subset of messages) where each included [message](#message) also includes its referenced messages.

## Network Layer

This layer manages the lower layers of internet communication like TCP. It is the most technical, and in some ways the least interesting. In this layer,  the autopeering, peer discovery modules ,and the gossip protocol manage the connections between [nodes](#node).

### Peering

The procedure of discovering and connecting to other network [nodes](#node).

### Neighbors

Network [nodes](#node) that are directly connected and can exchange [messages](#message) without intermediate nodes.

### Node

A machine which is part of the IOTA network. Its role is to issue new [transactions](#transaction) and to validate existing ones.

### Small-world Network

A network in which most [nodes](#node) can be reached from every other node in a few intermediate steps.

### Eclipse Attack

A cyber-attack that aims to isolate and attack a specific user, rather than the whole network.

### Splitting Attacks

An attack in which a malicious [node](#node) attempts to split the [Tangle](#tangle) into two branches. As one of the branches grows, the attacker publishes [transactions](#transaction) on the other branch to keep both alive. Splitting attacks attempt to slow down the [consensus](#consensus) process, or conduct a double spend.

### Sybil Attack

An attempt to gain control over a peer-to-peer network by forging multiple fake identities.

## Communication Layer

This layer stores and communicates information. This layer contains the Tangle, or “distributed ledger”. This layer also contains the rate control and timestamps.

### Message

The object that is gossiped between [neighbors](#neighbors).  The most basic unit of information of the IOTA Protocol. Each message has a type and size and contains data. A message includes all gossiped information.

### Message Overhead

The additional information (metadata) that needs to be sent along with the actual information (data). This can contain signatures, voting, heartbeat signals, and anything that is transmitted over the network but is not the [transaction](#transaction) itself.

### Parent

A [message](#message) that approves another message becomes its parent. A parent can be selected as _strong_ or _weak_ parent. If the parents past cone is _liked_, that pared is a strong parent. If the message is _liked_, but its past cone is _disliked_, the message is a weak parent. This mechanism is called [approval switch](#approval-switch).

### Payload

A field in a [message](#message) which determines the type. Some examples are:

* Value payload (type TransactionType).
* FPC Opinion payload (type StatementType).
* dRNG payload.
* Salt declaration payload.
* Generic data payload.

### Mana

The reputation of a [node](#node)  is based on a virtual token called _mana_. This reputation, working as a Sybil protection mechanism, is important for issuing more [transactions](#transaction), and having a higher influence during the voting process.

#### Epoch

A time interval that is used for a certain type of consensus [mana](#mana). At the end of each epoch, a snapshot of the state of mana distribution in the network is taken. Since this tool employs the timestamp of [messages](#message), every [node](#node) can reach [consensus](#consensus) on an epoch's mana distribution eventually.

### Transaction

A [message](#message) with [payload](#payload) of type _TransactionType_. It contains the information regarding a fund transfer.

#### UTXO

Unspent [transaction](#transaction) output.

#### Orphan

A [transaction](#transaction), or block, that is not referenced by any succeeding transaction, or block. An orphan is not considered confirmed, and will not be part of the [consensus](#consensus).

#### Reattachment

Resending a [transaction](#transaction) by redoing [tip selection](#tip-selection) and referencing newer tips by redoing [PoW](#proof-of-work).

#### History

The list of [transactions](#transaction) directly or indirectly approved by a given transaction.

#### Solidification Time

The solidification time is the point at which the entire [history](#history) of a [transaction](#transaction) has been received by a [node](#node).

#### Finality

This is the moment when the parties involved in a transfer can consider the deal done. Once a [transaction](#transaction) is completed there, is no way to revert or alter finality. Finality can be deterministic or probabilistic.

### Tip Selection

The process of selecting previous [messages](#message) to be referenced by a new message. These references are where a message attaches to the existing data structure. IOTA only enforces that a message approves at least two other messages, but the user controls the tip selection strategy,  with a good default provided by IOTA.

#### Tip

A message that has not yet been approved.

#### Local modifiers

Custom conditions that [nodes](#node) can take into account during [tip selection](#tip-selection). In IOTA, nodes do not necessarily have the same view of the Tangle; various kinds of information only locally available to them can be used to strengthen security.

#### Approval switch

When selecting a [message](#message) as a [parent](#parent), we can select from the strong or weak [tip](#tip) pool. This mechanism is called approval switch.

#### Approval weight

A [message](#message) gains [mana](#mana) weight, by other messages approving it directly or indirectly. However, only strong [parents](#parent) can propagate the mana weight to the past, while weak parents obtain the weight from its weak children but do not propagate it.

## Application Layer

The IOTA Protocol allows for a host of applications to run on the [message](#message) tangle. Anybody can design an application, and users can decide which applications to run on their [nodes](#node). These applications will all use the [communication layer](#communication-layer) to broadcast and store data.

### Core Applications

Applications that are necessary for the protocol to operate. These include for example:

* [The value transfer application](#value-transfer-application)
* The distributed random number generator (DRNG for short)
* The Fast Probabilistic Consensus (FPC) protocol

### Value Transfer Application

The application which maintains the ledger state.

### Faucet

A test application issuing funds on request.


## Markers

A tool that exists only locally, and allows performing certain calculations more efficiently. These calculations include [approval weight](#approval-weight) calculation or the existence of certain [messages](#message) in the past or future cone of another message.