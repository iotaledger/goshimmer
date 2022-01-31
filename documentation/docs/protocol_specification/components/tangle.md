---
description: The Tangle represents a growing partially-ordered set of messages, linked with each other through cryptographic primitives, and replicated to all nodes in the peer-to-peer network. It enables the ledger state (i.e., the UTXO-DAG formed by transactions contained in messages), and the possibility to store data.
image: /img/protocol_specification/tangle.png
keywords:
- message
- strong parents
- node
- transactions
- level of knowledge
- cone
- past
- future
- strong message 
- weak message
- approval weight 
---
# Tangle

## Data Types

| Name         | Description                                                                                                    |
| ------------ | -------------------------------------------------------------------------------------------------------------- |
| uint8        | An unsigned 8 bit integer encoded in Little Endian.                                                            |
| uint16       | An unsigned 16 bit integer encoded in Little Endian.                                                           |
| uint32       | An unsigned 32 bit integer encoded in Little Endian.                                                           |
| uint64       | An unsigned 64 bit integer encoded in Little Endian.                                                           |
| ByteArray[N] | A static size array of size N.                                                                                 |
| ByteArray    | A dynamically sized array. A uint32 denotes its length.                                                        |
| string       | A dynamically sized array of an UTF-8 encoded string. A uint16 denotes its length.                             |
| time         | Unix time in nanoseconds stored as `int64`, i.e., the number of nanoseconds elapsed since January 1, 1970 UTC. |

## Subschema Notation

| Name           | Description                                               |
| :------------- | :-------------------------------------------------------- |
| oneOf          | One of the listed subschemas.                             |
| optOneOf       | Optionally one of the listed subschemas.                  |
| anyOf          | Any (one or more) of the listed subschemas.               |
| `between(x,y)` | Between (but including) x and y of the listed subschemas. |

## Parameters

- `MAX_MESSAGE_SIZE=64 KB` The maximum allowed message size.
- `MAX_PAYLOAD_SIZE=65157 B` The maximum allowed payload size.
- `MIN_STRONG_PARENTS=1` The minimum amount of strong parents a message needs to reference.
- `MAX_PARENTS=8` The maximum amount of parents a message can reference.

## General Concept

[![The Tangle](/img/protocol_specification/tangle.png)](/img/protocol_specification/tangle.png)

The Tangle represents a growing partially-ordered set of messages, linked with each other through cryptographic primitives, and replicated to all nodes in the peer-to-peer network. The Tangle enables the ledger state (i.e., the UTXO-DAG formed by transactions contained in messages), and the possibility to store data.

### Terminology

- **Genesis**: The genesis message is used to bootstrap the Tangle and  creates the entire token supply and no other tokens will ever be created. It is the first message and does not have parents. It is marked as solid, eligible and confirmed.
- **Past cone**: All messages that are directly or indirectly referenced by a message are called its past cone.
- **Future cone**: All messages that directly or indirectly reference a message are called its future cone.
- **Solidity**: A message is marked as solid if its entire past cone until the Genesis (or the latest snapshot) is known.
- **Parents**: A message directly references between 1-8 previous messages that we call its **parents**. A parent can be either **strong** or **weak** (see [approval switch](#orphanage--approval-switch)).
- **Approvers**: Parents are approved by their referencing messages called **approvers**. It is thus a reverse mapping of parents. As in the parents' definition, an approver might be either **strong** or **weak**.
- **Branch**: A version of the ledger that temporarily coexists with other versions, each spawned by conflicting transactions. 

## Messages

Messages are created and signed by nodes. Next to several fields of metadata, they carry a **payload**. The maximum message size is `MAX_MESSAGE_SIZE`.

### Message ID

BLAKE2b-256 hash of the byte contents of the message. It should be used by the nodes to index the messages and by external APIs.

### Message structure
<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>Version</td>
        <td>uint8</td>
        <td>The message version. The schema specified in this RFC is for version <strong>1</strong> only. </td>
    </tr>
    <tr>
        <td>Parents blocks count</td>
        <td>uint8</td>
        <td>The amount of parents block preceding the current message.</td>
    </tr>
    <tr>
        <td valign="top">Parents Blocks <code>anyOf</code></td>
        <td colspan="2">
            <details open="true">
                <summary>Strong Parents Block</summary>
                <blockquote>
                Defines a parents block containing strong parents references.
                </blockquote>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>Parent Type</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>value 0</strong> to denote a <i>Strong Parents Block</i>.
                        </td>
                    </tr>
                    <tr>
                        <td>Parent Count</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>number</strong> of parent references in this block.
                        </td>
                    </tr>
                    <tr>
                        <td>Reference <code>between(1,8)</code></td>
                        <td>ByteArray[32]</td>
                        <td>Reference to a Message ID.</td>
                    </tr>
                </table>
            </details>
            <details open="true">
                <summary>Weak Parents Block</summary>
                <blockquote>
                Defines a parents block containing weak parents references.
                </blockquote>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>Parent Type</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>value 1</strong> to denote a <i>Weak Parents Block</i>.
                        </td>
                    </tr>
                    <tr>
                        <td>Parent Count</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>number</strong> of parent references in this block.
                        </td>
                    </tr>
                    <tr>
                        <td>Reference <code>between(1,8)</code></td>
                        <td>ByteArray[32]</td>
                        <td>Reference to a Message ID.</td>
                    </tr>
                </table>
            </details>
            <details open="true">
                <summary>Dislike Parents Block</summary>
                <blockquote>
                Defines a parents block containing dislike parents references.
                </blockquote>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>Parent Type</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>value 2</strong> to denote a <i>Dislike Parents Block</i>.
                        </td>
                    </tr>
                    <tr>
                        <td>Parent Count</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>number</strong> of parent references in this block.
                        </td>
                    </tr>
                    <tr>
                        <td>Reference <code>between(1,8)</code></td>
                        <td>ByteArray[32]</td>
                        <td>Reference to a Message ID.</td>
                    </tr>
                </table>
            </details>
            <details open="true">
                <summary>Like Parents Block</summary>
                <blockquote>
                Defines a parents block containing like parents references.
                </blockquote>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>Parent Type</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>value 3</strong> to denote a <i>Like Parents Block</i>.
                        </td>
                    </tr>
                    <tr>
                        <td>Parent Count</td>
                        <td>uint8</td>
                        <td>
                            Set to <strong>number</strong> of parent references in this block.
                        </td>
                    </tr>
                    <tr>
                        <td>Reference <code>between(1,8)</code></td>
                        <td>ByteArray[32]</td>
                        <td>Reference to a Message ID.</td>
                    </tr>
                </table>
            </details>
        </td>
    </tr>
    <tr>
        <td>Issuer public key (Ed25519)</td>
        <td>ByteArray[32]</td>
        <td>The public key of the node issuing the message.</td>
    </tr>
    <tr>
        <td>Issuing time</td>
        <td>time</td>
        <td>The time the message was issued.</td>
    </tr>
    <tr>
        <td>Sequence number</td>
        <td>uint64</td>
        <td>The always increasing number of issued messages of the issuing node.</td>
    </tr>
    <tr>
        <td>Payload length</td>
        <td>uint32</td>
        <td>The length of the Payload. Since its type may be unknown to the node, it must be declared in advance. 0 length means no payload will be attached.</td>
    </tr>
    <tr>
        <td colspan="1">
            Payload
        </td>
        <td colspan="2">
            <details open="true">
                <summary>Generic Payload</summary>
                <blockquote>
                An outline of a general payload
                </blockquote>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>Payload Type</td>
                        <td>uint32</td>
                        <td>
                            The type of the payload. It will instruct the node how to parse the fields that follow. Types in the range of 0-127 are "core types", that all nodes are expected to know.
                        </td>
                    </tr>
                    <tr>
                        <td>Data Fields</td>
                        <td>ANY</td>
                        <td>A sequence of fields, where the structure depends on <code>payload type</code>.</td>
                    </tr>
                </table>
            </details>
        </td>
    </tr>
    <tr>
        <td>Nonce</td>
        <td>uint64</td>
        <td>The nonce which lets this message fulfill the adaptive Proof-of-Work requirement.</td>
    </tr>
    <tr>
        <td>Signature (Ed25519)</td>
        <td>ByteArray[64]</td>
        <td>Signature of the issuing node's private key signing the entire message bytes.</td>
    </tr>
</table>


### Syntactical Validation

Messages that do no pass the Syntactical Validation are discarded. Only syntactically valid messages continue in the data flow, i.e., pass to the Semantic Validation.

A message is syntactically valid if:
1. The message length does not exceed `MAX_MESSAGE_SIZE` bytes.
1. When the message parsing is complete, there are not any trailing bytes left that were not parsed.
1. Parents Blocks must be ordered by ASC type with no repetitions.
1. A <i>Strong Parents Block</i> must exist.
1. There must be at least 1 parent per block and no more than 8.
1. Parents in each Parents Block types must be ordered ASC without repetition. 
1. Parents must be unique across Parents Blocks. But there may be repetitions across the <i>Strong</i> and <i>Liked</i> blocks.
1. <i>Parents Block Count</i> and <i>Parents Count</i> must match the actual number of blocks and parents respectively.

### Semantic Validation

Messages that do not pass the Semantic Validation are discarded. Only semantically valid messages continue in the data flow.

A message is semantically valid if:
1. The Message PoW Hash contains at least the number of leading 0 defined as required by the PoW.
2. The signature of the issuing node is valid.
3. It passes [parents age checks](#age-of-parents).


#### Votes Validation

1. Only one dislike parent is allowed per conflict set.
1. Only one like parent is allowed per conflict set.
1. Every dislike parent must be in the past cone of a strong parent.
1. For each like parent, only one dislike parent must exist pointing to a message containing a transaction within the same conflict set.
1. For every dislike parent and for every conflict set it belongs to, a like parent must also exist pointing to a message within the considered conflict set, provided that such transaction is not already present in the past cone of any strong parent.
1. For each referenced conflict set, from all parents types, for each referenced conflict set, must result in only a single transaction support.
1. Only one like or weak parent can be within the same conflict set.



## Payloads

Payloads can contain arbitrary data up to `MAX_PAYLOAD_SIZE`, which allows building additional protocols on top of the base protocol in the same way as TCP/IP allows to define additional protocols on top of its generic data segment.

Payloads can recursively contain other payloads, which enables the creation of higher level protocols based on the same concepts of layers, as in traditional software and network architecture.

Payloads other than transactions are always liked with level of knowledge 3.

### User-defined Payloads

A node can choose to interpret user-defined payloads by listenting to its specific **payload type** (possibly via third-party code/software). If a node does not know a certain **payload type**, it simply treats it as arbitrary data.

### Core Payloads

The core protocol defines a number of payloads that every node needs to interpret and process in order to participate in the network.

- **Transactions:** Value transfers that constitute the ledger state.
- **Data:**  Pure data payloads allow to send unsigned messages.
- **dRNG:** Messages that contain randomness or committee declarations.

## Solidification

Due to the asynchronicity of the network, we may receive messages for which their past cone is not known yet. We refer to these messages as unsolid messages. It is not possible neither to approve nor to gossip unsolid messages. The actions required to obtain such missing messages is called solidification.
**Solidification** is the process of requesting missing referenced messages. It may be recursively repeated until all of a message's past cone up to the genesis (or snapshot) becomes solid.

In that way, the Tangle enables all nodes to retrieve all of a message's history, even the ones joining the network at a point later in time.

### Definitions

* **valid**: A message is considered valid if it passes the following filters from the solidifier and from the message booker:
    * solidifier: it checks if parents are valid,
    * booker: it checks if the contained transaction is valid. Notice that only messages containing a transaction are required to perform this check.
* **parents age check**: A check that ensures the timestamps of parents and child are valid, following the details defined in the [Timestamp specification](#age-of-parents).
* **solid**: A message is solid if it passes parents age check and all its parents are stored in the storage, solid and valid.

### Detailed Design

During solidification, if a node is missing a referenced message, the corresponding message ID is stored in the `solidification buffer`. A node asks its neighbors for the missing message by sending a `solidification request` containing the message ID. Once the requested message is received from its neighbors, its message ID shall be removed from the `solidification buffer`. The requested message is marked as solid after it passes the standard solidification checks. If any of the checks fails, the message remains unsolid.

If a message gets solid, it shall walk through the rest of the data flow, then propagate the solid status to its future cone by performing the solidification checks on each of the messages in its future cone again.

[![Message solidification specs](/img/protocol_specification/GoShimmer-flow-solidification_spec.png)](/img/protocol_specification/GoShimmer-flow-solidification_spec.png)


## Orphanage & Approval Switch

The Tangle builds approval of a given message by directly or indirectly attaching other messages in its future cone. Due to different reasons, such as the TSA not picking up a given message before its timestamp is still *fresh* or because its past cone has been rejected, a message can become orphan. This implies that the message cannot be included in the Tangle history since all the recent tips do not contain it in their past cone and thus, it cannot be retrieved during solidification. As a result, it might happen that honest messages and transactions would need to be reissued or reattached.
To overcome this limitation, we propose the `approval switch`. The idea is to minimize honest messages along with transactions getting orphaned, by assigning a different meaning to the parents of a message.

### Detailed design

Each message can express two levels of approval with respect to its parents:
* **Strong**: it defines approval for both the referenced message along with its entire past cone.
* **Weak**: it defines approval for the referenced message but not for its past cone.

Let's consider the following example:

[![Detailed Design Example](/img/protocol_specification/detailed_desing.png "Detailed Design Example")](/img/protocol_specification/detailed_desing.png)

Message *D* contains a transaction that has been rejected, thus, due to the monotonicity rule, its future cone must be orphaned. Both messages *F* (transaction) and *E* (data) directly reference *D* and, traditionally, they should not be considered for tip selection. However, by introducing the approval switch, these messages can be picked up via a **weak** reference as messages *G* and *H* show.

We define two categories of eligible messages:
- **Strong message**:
    * It is eligible
    * Its payload is liked with level of knowledge >=2
    * Its branch is **liked** with level of knowledge >= 2
- **Weak message**:
    * It is eligible
    * Its payload is liked with level of knowledge >=2
    * Its branch is **not liked** with level of knowledge >= 2

We call *strong approver of x* (or *strong child of x*) any strong message *y* approving *x* via a strong reference. Similarly, we call *weak approver of x* (or *weak child of x*) any strong message *y* approving *x* via a weak reference.

### TSA

We define two separate tip types:
* **Strong tip**:
    * It is a strong message
    * It is not directly referenced by any strong message via strong parent
* **Weak tip**:
    * It is a weak message
    * It is not directly referenced by any strong message via weak parent

Consequently, a node keeps track of the tips by using two distinct tips sets:
* **Strong tips set**: contains the strong tips
* **Weak tips set**: contains the weak tips

Tips of both sets must be managed according to the local perception of the node. Hence, a strong tip loses its tip status if it gets referenced (via strong parent) by a strong message. Similarly, a weak tip loses its tip status if it gets referenced (via weak parent) by a strong message. This means that weak messages approving via either strong or weak parents, do not have an impact on the tip status of the messages they reference.

### Branch Management

A message inherits the branch of its strong parents, while it does not inherit the branch of its weak parents.

#### Approval Weight

The approval weight of a given message takes into account all of its future cone built over all its strong approvers.
Let's consider the following example:

[![Approval Weight](/img/protocol_specification/approval_weight_example.png "Approval Weight")](/img/protocol_specification/approval_weight_example.png )

*E* is a weak message strongly approving *B* and *D*. When considering the approval weight of *B*, only the strong approvers of its future cone are used, thus, *D, E, F*. Note that, the approval weight of *E* would instead be built over *G, H, I*. Therefore, its approval weight does not add up to its own weight (for instance, when looking at the approval weight of *B*).

### Solidification

The solidification process does not change, both parent types are used to progress.

### Test cases

* message *x* strongly approves a strong message *y*: ok
* message *x* weakly approves a strong message *y*: it's weird, counts for approval weight of *y* but does not affect the tip status of *y*
* message *x* strongly approves a weak message *y*: *x* becomes a weak message
* message *x* weakly approves a weak message *y*: ok


## Finality

Users need to know whether their information will not be orphaned. However, finality is inherently probabilistic. For instance, consider the following scenario: an attacker can trivially maintain a chain of messages that do not approve any other message. At any given point in time, it is possible that all messages will be orphaned except this chain. This is incredibly unlikely, but yet still possible.

Therefore, we introduce [Approval Weight](consensus_mechanism.md#approval-weight-aw) to measure the finality of any given message. Similarly to Bitcoin's 6 block rule, AW describes how deeply buried a message in the Tangle is. If a message reaches >50% of active consensus mana approving it, i.e., its future cone contains messages of nodes that together assert >50% of active consensus mana, it as finalized and, thus, confirmed. Specifically, in GoShimmer we use [markers](markers.md) to optimize AW calculations and approximate AW instead of tracking it for each message individually.

## Timestamps

In order to enable snapshotting based on time constraints rather than special messages in the Tangle (e.g. checkpoints), nodes need to share the same perception of time. Specifically, they need to have consensus on the *age of messages*. This is one of the reasons that messages must contain a field `timestamp` which represents the creation time of the message and is signed by the issuing node.

Having consensus on the creation time of messages enables not only total ordering but also new applications that require certain guarantees regarding time. Specifically, we use message timestamps to enforce timestamps in transactions, which may also be used in computing the Mana associated to a particular node ID.


### Clock Synchronization

Nodes need to share a reasonably similar perception of time in order to effectively judge the accuracy of timestamps. Therefore, we propose that nodes synchronize their clock on startup and resynchronize periodically every `30min` to counter [drift](https://en.wikipedia.org/wiki/Clock_drift) of local clocks. Instead of changing a nodes' system clock, we introduce an `offset` parameter to adjust for differences between *network time* and local time of a node. Initially, the [Network Time Protocol (NTP)](https://en.wikipedia.org/wiki/Network_Time_Protocol) ([Go implementation](https://github.com/beevik/ntp)) is used to achieve this task.

### General Timestamp Rules

Every message contains a timestamp, which is signed by the issuing node. Thus, the timestamp itself is objective and immutable.  Furthermore, transactions also contain a timestamp, which is also signed by the sender of the transaction (user) and thus immutable. We first discuss the rules regarding message timestamps.

In order for a message to be eligible for tip selection, the timestamp of every message in its past cone (both weak and strong) must satisfy certain requirements. These requirements fall into two categories: objective and subjective. The objective criteria only depend on information written directly in the Tangle and are applied immediately upon solidification.  Thus, all nodes immediately have consensus on the objective criteria.  In this section, we will discuss these objective criteria.

The quality of the timestamp is a subjective criterion since it is based on the solidification time of the message.  Thus, nodes must use a consensus algorithm, to decide which messages should be rejected based on subjective criteria. However, currently this feature is not yet implemented in GoShimmer, and we assume all timestamps to be good.

### Age of parents
It is problematic when incoming messages reference extremely old messages. If any new message may reference any message in the Tangle, then a node will need to keep all messages readily available, precluding snapshotting. For this reason, we require that the difference between the timestamp of a message, and the timestamp of its parents must be at most `30min`. Additionally, we require that timestamps are monotonic, i.e., parents must have a timestamp smaller than their children's timestamps.


### Message timestamp vs transaction timestamp
Transactions contain a timestamp that is signed by the user when creating the transaction. It is thus different from the timestamp in the message which is created and signed by the node. We require
```go
transaction.timestamp+TW >= message.timestamp >= transaction.timestamp
```
where `TW` defines the maximum allowed difference between both timestamps, currently set to `10min`.

If a node receives a transaction from a user with an invalid timestamp it does not create a message but discards the transaction with a corresponding error message to the user. To prevent a user's local clock differences causing issues the node should offer an API endpoint to retrieve its `SyncedTime` according to the network time.

### Reattachments
Reattachments of a transaction are possible during the time window `TW`. Specifically, a transaction may be reattached in a new message as long as the condition `message.timestamp-TW >= transaction.timestamp` is fulfilled. If for some reason a transaction is not *picked up* (even after reattachment) and thus being orphaned, the user needs to create a new transaction with a current timestamp.

### Age of UTXO
Inputs to a transaction (unspent outputs) inherit their spent time from the transaction timestamp. Similarly, unspent outputs inherit their creation time from the transaction timestamp as well. For a transaction to be considered valid we require

```go
transaction.timestamp >= inputs.timestamp
```
In other words, all inputs to a transaction need to have a smaller or equal timestamp than the transaction. In turn, all created unspent outputs will have a greater or equal timestamp than all inputs.


## Tangle Time
For a variety of reasons, a node needs to be able to determine if it is in sync with the rest of the network, including the following:
- to signal to clients that its perception is healthy,
- to know when to issue messages (nodes out of sync should not issue messages, lest they are added to the wrong part of the Tangle),
- to schedule messages at the correct rate: out of sync nodes should schedule faster in order to catch up with the network.

Every DLT is a clock, or more specifically a network of synchronized clocks. This clock has a natural correspondence with "real time". If the DLT clock differs significantly from local time, then we can conclude that our DLT clock is off from all the other clocks, and thus the node is out of sync.

Tangle time is the timestamp of the last confirmed message. It cannot be attacked without controlling enough mana to accept incorrect timestamps, making it a reliable, attack-resistant quantity.

Typically speaking, `CurrentTime - TangleTime` is, on average, the  approximate confirmation time of messages.  Thus, if this difference is too far off, then we can conclude that we do not know which messages are confirmed and thus we are out of sync.  In this spirit, we are able to define the following function.

```go
func Synced() bool {
  if CurrentTime - TangleTime <= SYNC_THRESHOLD {
    return true
  }
  
  return false
}
```

The following figure displays the Tangle Time visually: 
[![Tangle Time](/img/protocol_specification/tangle_time.jpg "Tangle Time")](/img/protocol_specification/tangle_time.jpg )
