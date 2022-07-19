---
description: The Tangle represents a growing partially-ordered set of blocks, linked with each other through cryptographic primitives, and replicated to all nodes in the peer-to-peer network. It enables the ledger state (i.e., the UTXO-DAG formed by transactions contained in blocks), and the possibility to store data.
image: /img/protocol_specification/tangle.png
keywords:
- block
- strong parents
- node
- transactions
- level of knowledge
- cone
- past
- future
- strong block 
- weak block
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

- `MAX_MESSAGE_SIZE=64 KB` The maximum allowed block size.
- `MAX_PAYLOAD_SIZE=65157 B` The maximum allowed payload size.
- `MIN_STRONG_PARENTS=1` The minimum amount of strong parents a block needs to reference.
- `MAX_PARENTS=8` The maximum amount of parents a block can reference.

## General Concept

[![The Tangle](/img/protocol_specification/tangle.png)](/img/protocol_specification/tangle.png)

The Tangle represents a growing partially-ordered set of blocks, linked with each other through cryptographic primitives, and replicated to all nodes in the peer-to-peer network. The Tangle enables the ledger state (i.e., the UTXO-DAG formed by transactions contained in blocks), and the possibility to store data.

### Terminology

- **Genesis**: The genesis block is used to bootstrap the Tangle and  creates the entire token supply and no other tokens will ever be created. It is the first block and does not have parents. It is marked as solid, eligible and confirmed.
- **Past cone**: All blocks that are directly or indirectly referenced by a block are called its past cone.
- **Future cone**: All blocks that directly or indirectly reference a block are called its future cone.
- **Solidity**: A block is marked as solid if its entire past cone until the Genesis (or the latest snapshot) is known.
- **Parents**: A block directly references between 1-8 previous blocks that we call its **parents**. A parent can be either **strong** or **weak** (see [approval switch](#orphanage--approval-switch)).
- **Children**: Parents are approved by their referencing blocks called **children**. It is thus a reverse mapping of parents. As in the parents' definition, an child might be either **strong** or **weak**.
- **Conflict**: A version of the ledger that temporarily coexists with other versions, each spawned by conflicting transactions. 

## Blocks

Blocks are created and signed by nodes. Next to several fields of metadata, they carry a **payload**. The maximum block size is `MAX_MESSAGE_SIZE`.

### Block ID

BLAKE2b-256 hash of the byte contents of the block. It should be used by the nodes to index the blocks and by external APIs.

### Block structure
<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>Version</td>
        <td>uint8</td>
        <td>The block version. The schema specified in this RFC is for version <strong>1</strong> only. </td>
    </tr>
    <tr>
        <td>Parents blocks count</td>
        <td>uint8</td>
        <td>The amount of parents block preceding the current block.</td>
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
                        <td>Reference to a Block ID.</td>
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
                        <td>Reference to a Block ID.</td>
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
                        <td>Reference to a Block ID.</td>
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
                        <td>Reference to a Block ID.</td>
                    </tr>
                </table>
            </details>
        </td>
    </tr>
    <tr>
        <td>Issuer public key (Ed25519)</td>
        <td>ByteArray[32]</td>
        <td>The public key of the node issuing the block.</td>
    </tr>
    <tr>
        <td>Issuing time</td>
        <td>time</td>
        <td>The time the block was issued.</td>
    </tr>
    <tr>
        <td>Sequence number</td>
        <td>uint64</td>
        <td>The always increasing number of issued blocks of the issuing node.</td>
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
        <td>The nonce which lets this block fulfill the adaptive Proof-of-Work requirement.</td>
    </tr>
    <tr>
        <td>Signature (Ed25519)</td>
        <td>ByteArray[64]</td>
        <td>Signature of the issuing node's private key signing the entire block bytes.</td>
    </tr>
</table>


### Syntactical Validation

Blocks that do no pass the Syntactical Validation are discarded. Only syntactically valid blocks continue in the data flow, i.e., pass to the Semantic Validation.

A block is syntactically valid if:
1. The block length does not exceed `MAX_MESSAGE_SIZE` bytes.
1. When the block parsing is complete, there are not any trailing bytes left that were not parsed.
1. Parents Blocks must be ordered by ASC type with no repetitions.
1. A <i>Strong Parents Block</i> must exist.
1. There must be at least 1 parent per block and no more than 8.
1. Parents in each Parents Block types must be ordered ASC without repetition. 
1. Parents must be unique across Parents Blocks. But there may be repetitions across the <i>Strong</i> and <i>Liked</i> blocks.
1. <i>Parents Block Count</i> and <i>Parents Count</i> must match the actual number of blocks and parents respectively.

### Semantic Validation

Blocks that do not pass the Semantic Validation are discarded. Only semantically valid blocks continue in the data flow.

A block is semantically valid if:
1. The Block PoW Hash contains at least the number of leading 0 defined as required by the PoW.
2. The signature of the issuing node is valid.
3. It passes [parents age checks](#age-of-parents).


#### Votes Validation

1. Only one dislike parent is allowed per conflict set.
1. Only one like parent is allowed per conflict set.
1. Every dislike parent must be in the past cone of a strong parent.
1. For each like parent, only one dislike parent must exist pointing to a block containing a transaction within the same conflict set.
1. For every dislike parent and for every conflict set it belongs to, a like parent must also exist pointing to a block within the considered conflict set, provided that such transaction is not already present in the past cone of any strong parent.
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
- **Data:**  Pure data payloads allow to send unsigned blocks.
- **dRNG:** Blocks that contain randomness or committee declarations.

## Solidification

Due to the asynchronicity of the network, we may receive blocks for which their past cone is not known yet. We refer to these blocks as unsolid blocks. It is not possible neither to approve nor to gossip unsolid blocks. The actions required to obtain such missing blocks is called solidification.
**Solidification** is the process of requesting missing referenced blocks. It may be recursively repeated until all of a block's past cone up to the genesis (or snapshot) becomes solid.

In that way, the Tangle enables all nodes to retrieve all of a block's history, even the ones joining the network at a point later in time.

### Definitions

* **valid**: A block is considered valid if it passes the following filters from the solidifier and from the block booker:
    * solidifier: it checks if parents are valid,
    * booker: it checks if the contained transaction is valid. Notice that only blocks containing a transaction are required to perform this check.
* **parents age check**: A check that ensures the timestamps of parents and child are valid, following the details defined in the [Timestamp specification](#age-of-parents).
* **solid**: A block is solid if it passes parents age check and all its parents are stored in the storage, solid and valid.

### Detailed Design

During solidification, if a node is missing a referenced block, the corresponding block ID is stored in the `solidification buffer`. A node asks its neighbors for the missing block by sending a `solidification request` containing the block ID. Once the requested block is received from its neighbors, its block ID shall be removed from the `solidification buffer`. The requested block is marked as solid after it passes the standard solidification checks. If any of the checks fails, the block remains unsolid.

If a block gets solid, it shall walk through the rest of the data flow, then propagate the solid status to its future cone by performing the solidification checks on each of the blocks in its future cone again.

[![Block solidification specs](/img/protocol_specification/GoShimmer-flow-solidification_spec.png)](/img/protocol_specification/GoShimmer-flow-solidification_spec.png)


## Orphanage & Approval Switch

The Tangle builds approval of a given block by directly or indirectly attaching other blocks in its future cone. Due to different reasons, such as the TSA not picking up a given block before its timestamp is still *fresh* or because its past cone has been rejected, a block can become orphan. This implies that the block cannot be included in the Tangle history since all the recent tips do not contain it in their past cone and thus, it cannot be retrieved during solidification. As a result, it might happen that honest blocks and transactions would need to be reissued or reattached.
To overcome this limitation, we propose the `approval switch`. The idea is to minimize honest blocks along with transactions getting orphaned, by assigning a different meaning to the parents of a block.

### Detailed design

Each block can express two levels of approval with respect to its parents:
* **Strong**: it defines approval for both the referenced block along with its entire past cone.
* **Weak**: it defines approval for the referenced block but not for its past cone.

Let's consider the following example:

[![Detailed Design Example](/img/protocol_specification/detailed_desing.png "Detailed Design Example")](/img/protocol_specification/detailed_desing.png)

Block *D* contains a transaction that has been rejected, thus, due to the monotonicity rule, its future cone must be orphaned. Both blocks *F* (transaction) and *E* (data) directly reference *D* and, traditionally, they should not be considered for tip selection. However, by introducing the approval switch, these blocks can be picked up via a **weak** reference as blocks *G* and *H* show.

We define two categories of eligible blocks:
- **Strong block**:
    * It is eligible
    * Its payload is liked with level of knowledge >=2
    * Its conflict is **liked** with level of knowledge >= 2
- **Weak block**:
    * It is eligible
    * Its payload is liked with level of knowledge >=2
    * Its conflict is **not liked** with level of knowledge >= 2

We call *strong child of x* (or *strong child of x*) any strong block *y* approving *x* via a strong reference. Similarly, we call *weak child of x* (or *weak child of x*) any strong block *y* approving *x* via a weak reference.

### Tip Pool and Time Since Confirmation Check

When a block is scheduled, it is gossiped to the node's neighbors and, normally, added to the local tip pool 
except in the following situations:

* A confirmed block shall not be added to the tip pool (it shall be skipped by the scheduler).
* A block that has confirmed or scheduled children shall not be added to the tip pool.

Additionally, strong parents of a block are removed from the tip pool, when the block is added and unused tips are removed from the tip pool after a certain amount of time.

When selecting tips from the tip pool an additional check is performed to make sure that the timestamp and the
past cone of a selected block is valid. For the selected tip, the algorithm needs to find a timestamp of the oldest parent of the oldest
unconfirmed block in the past cone of the tip (`TS_oum`). If the difference between current Confirmed Tangle Time `now` and the
timestamp of the oldest confirmed block is greater than a certain threshold (`now - TS_oum > TSC_threshold`), then
the tip cannot be selected and another one needs to be found. The tip stays in the tip pool until it is
automatically removed because of its age.

The Time Since Confirmation check solves the mention problem of [false positive schedule](congestion_control.md#false-positive-schedule)
by eventually orphaning blocks that were dropped by the network.

### Conflict Management

A block inherits the conflict of its strong parents, while it does not inherit the conflict of its weak parents.

#### Approval Weight

The approval weight of a given block takes into account all of its future cone built over all its strong children.
Let's consider the following example:

[![Approval Weight](/img/protocol_specification/approval_weight_example.png "Approval Weight")](/img/protocol_specification/approval_weight_example.png )

*E* is a weak block strongly approving *B* and *D*. When considering the approval weight of *B*, only the strong children of its future cone are used, thus, *D, E, F*. Note that, the approval weight of *E* would instead be built over *G, H, I*. Therefore, its approval weight does not add up to its own weight (for instance, when looking at the approval weight of *B*).

### Solidification

The solidification process does not change, both parent types are used to progress.

### Test cases

* block *x* strongly approves a strong block *y*: ok
* block *x* weakly approves a strong block *y*: it's weird, counts for approval weight of *y* but does not affect the tip status of *y*
* block *x* strongly approves a weak block *y*: *x* becomes a weak block
* block *x* weakly approves a weak block *y*: ok


## Finality

Users need to know whether their information will not be orphaned. However, finality is inherently probabilistic. For instance, consider the following scenario: an attacker can trivially maintain a chain of blocks that do not approve any other block. At any given point in time, it is possible that all blocks will be orphaned except this chain. This is incredibly unlikely, but yet still possible.

Therefore, we introduce [Approval Weight](consensus_mechanism.md#approval-weight-aw) to measure the finality of any given block. Similarly to Bitcoin's 6 block rule, AW describes how deeply buried a block in the Tangle is. If a block reaches >50% of active consensus mana approving it, i.e., its future cone contains blocks of nodes that together assert >50% of active consensus mana, it as finalized and, thus, confirmed. Specifically, in GoShimmer we use [markers](markers.md) to optimize AW calculations and approximate AW instead of tracking it for each block individually.

## Timestamps

In order to enable snapshotting based on time constraints rather than special blocks in the Tangle (e.g. checkpoints), nodes need to share the same perception of time. Specifically, they need to have consensus on the *age of blocks*. This is one of the reasons that blocks must contain a field `timestamp` which represents the creation time of the block and is signed by the issuing node.

Having consensus on the creation time of blocks enables not only total ordering but also new applications that require certain guarantees regarding time. Specifically, we use block timestamps to enforce timestamps in transactions, which may also be used in computing the Mana associated to a particular node ID.


### Clock Synchronization

Nodes need to share a reasonably similar perception of time in order to effectively judge the accuracy of timestamps. Therefore, we propose that nodes synchronize their clock on startup and resynchronize periodically every `30min` to counter [drift](https://en.wikipedia.org/wiki/Clock_drift) of local clocks. Instead of changing a nodes' system clock, we introduce an `offset` parameter to adjust for differences between *network time* and local time of a node. Initially, the [Network Time Protocol (NTP)](https://en.wikipedia.org/wiki/Network_Time_Protocol) ([Go implementation](https://github.com/beevik/ntp)) is used to achieve this task.

### General Timestamp Rules

Every block contains a timestamp, which is signed by the issuing node. Thus, the timestamp itself is objective and immutable.  Furthermore, transactions also contain a timestamp, which is also signed by the sender of the transaction (user) and thus immutable. We first discuss the rules regarding block timestamps.

In order for a block to be eligible for tip selection, the timestamp of every block in its past cone (both weak and strong) must satisfy certain requirements. These requirements fall into two categories: objective and subjective. The objective criteria only depend on information written directly in the Tangle and are applied immediately upon solidification.  Thus, all nodes immediately have consensus on the objective criteria.  In this section, we will discuss these objective criteria.

The quality of the timestamp is a subjective criterion since it is based on the solidification time of the block.  Thus, nodes must use a consensus algorithm, to decide which blocks should be rejected based on subjective criteria. However, currently this feature is not yet implemented in GoShimmer, and we assume all timestamps to be good.

### Age of parents
It is problematic when incoming blocks reference extremely old blocks. If any new block may reference any block in the Tangle, then a node will need to keep all blocks readily available, precluding snapshotting. For this reason, we require that the difference between the timestamp of a block, and the timestamp of its parents must be at most `30min`. Additionally, we require that timestamps are monotonic, i.e., parents must have a timestamp smaller than their children's timestamps.


### Block timestamp vs transaction timestamp
Transactions contain a timestamp that is signed by the user when creating the transaction. It is thus different from the timestamp in the block which is created and signed by the node. We require
```go
transaction.timestamp+TW >= block.timestamp >= transaction.timestamp
```
where `TW` defines the maximum allowed difference between both timestamps, currently set to `10min`.

If a node receives a transaction from a user with an invalid timestamp it does not create a block but discards the transaction with a corresponding error block to the user. To prevent a user's local clock differences causing issues the node should offer an API endpoint to retrieve its `SyncedTime` according to the network time.

### Reattachments
Reattachments of a transaction are possible during the time window `TW`. Specifically, a transaction may be reattached in a new block as long as the condition `block.timestamp-TW >= transaction.timestamp` is fulfilled. If for some reason a transaction is not *picked up* (even after reattachment) and thus being orphaned, the user needs to create a new transaction with a current timestamp.

### Age of UTXO
Inputs to a transaction (unspent outputs) inherit their spent time from the transaction timestamp. Similarly, unspent outputs inherit their creation time from the transaction timestamp as well. For a transaction to be considered valid we require

```go
transaction.timestamp >= inputs.timestamp
```
In other words, all inputs to a transaction need to have a smaller or equal timestamp than the transaction. In turn, all created unspent outputs will have a greater or equal timestamp than all inputs.


## Tangle Time
For a variety of reasons, a node needs to be able to determine if it is in sync with the rest of the network, including the following:
- to signal to clients that its perception is healthy,
- to know when to issue blocks (nodes out of sync should not issue blocks, lest they are added to the wrong part of the Tangle),
- to schedule blocks at the correct rate: out of sync nodes should schedule faster in order to catch up with the network.

Every DLT is a clock, or more specifically a network of synchronized clocks. This clock has a natural correspondence with "real time". If the DLT clock differs significantly from local time, then we can conclude that our DLT clock is off from all the other clocks, and thus the node is out of sync.

Tangle time is the timestamp of the last confirmed block. It cannot be attacked without controlling enough mana to accept incorrect timestamps, making it a reliable, attack-resistant quantity.

Typically speaking, `CurrentTime - TangleTime` is, on average, the  approximate confirmation time of blocks.  Thus, if this difference is too far off, then we can conclude that we do not know which blocks are confirmed and thus we are out of sync.  In this spirit, we are able to define the following function.

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
