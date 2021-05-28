# Tangle

## Data types
| Name   | Description   |
| ------ | ------------- |
| uint8  | An unsigned 8 bit integer encoded in Little Endian. |
| uint16  | An unsigned 16 bit integer encoded in Little Endian. |
| uint32  | An unsigned 32 bit integer encoded in Little Endian. |
| uint64  | An unsigned 64 bit integer encoded in Little Endian. |
| ByteArray[N] | A static size array of size N.   |
| ByteArray | A dynamically sized array. A uint32 denotes its length.   |
| string | A dynamically sized array of an UTF-8 encoded string. A uint16 denotes its length.   |
| time    | Unix time in nanoseconds stored as `int64`, i.e., the number of nanoseconds elapsed since January 1, 1970 UTC. |

<summary>Subschema Notation</summary>
<table>
    <tr>
        <th>Name</th>
        <th>Description</th>
    </tr>
    <tr>
        <td><code>oneOf</code></td>
        <td>One of the listed subschemas.</td>
    </tr>
    <tr>
        <td><code>optOneOf</code></td>
        <td>Optionally one of the listed subschemas.</td>
    </tr>
    <tr>
        <td><code>anyOf</code></td>
        <td>Any (one or more) of the listed subschemas.</td>
    </tr>
    <tr>
        <td><code>between(x,y)</code></td>
        <td>Between (but including) x and y of the listed subschemas.</td>
    </tr>
</table>

## Parameters
- `MAX_MESSAGE_SIZE=64 KB` The maximum allowed message size.
- `MAX_PAYLOAD_SIZE=65157 B` The maximum allowed payload size.
- `MIN_STRONG_PARENTS=1` The minimum amount of strong parents a message needs to reference.

## General concept
![Tangle](https://i.ibb.co/RyqbZzN/tangle.png)

The Tangle represents a growing partially-ordered set of messages, linked with each other through cryptographic primitives, and replicated to all nodes in the peer-to-peer network. The Tangle enables the ledger state (i.e., the UTXO-DAG formed by transactions contained in messages), and the possibility to store data.

### Terminology
- **Genesis**: The genesis message is used to bootstrap the Tangle and  creates the entire token supply and no other tokens will ever be created. It is the first message and does not have parents. It is marked as solid, eligible and confirmed.
- **Past cone**: All messages that are directly or indirectly referenced by a message are called its past cone.
- **Future cone**: All messages that directly or indirectly reference a message are called its future cone.
- **Solidity**: A message is marked as solid if its entire past cone until the Genesis (or the latest snapshot) is known.
- **Parents**: A message directly references between 1-8 previous messages that we call its **parents**. A parent can be either **strong** or **weak** (see [approval switch](#orphanage--approval-switch)).
- **Approvers**: Parents are approved by their referencing messages called **approvers**. It is thus a reverse mapping of parents. As in the parents' definition, an approver might be either **strong** or **weak**.

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
        <td>Parents count</td>
        <td>uint8</td>
        <td>The amount of parents preceding the current message.</td>
    </tr>
    <tr>
        <td>Parents type</td>
        <td>uint8</td>
        <td>Bitwise encoding of parent type matching the order of preceding parents starting at <code>least significant bit</code>. <code>1</code> indicates a strong parent, while <code>0</code> signals a weak parent. At least <code>MIN_STRONG_PARENTS</code> parent type must be strong.</td>
    </tr>
    <tr>
        <td colspan="1">
            Parents <code>between(1,8)</code>
        </td>
        <td colspan="2">
            <details open="true">
                <summary>Parents, ordered by hash ASC</summary>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>Parent</td>
                        <td>ByteArray[32]</td>
                        <td>The Message ID of the <i>parent Message</i>.</td>
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
2. When the message parsing is complete, there are not any trailing bytes left that were not parsed.
4. At least 1 and at most 8 distinct parents are given, ordered ASC and at least `MIN_STRONG_PARENTS` are strong parents.

### Semantic Validation
Messages that do not pass the Semantic Validation are discarded. Only semantically valid messages continue in the data flow.

A message is semantically valid if:
1. The Message PoW Hash contains at least the number of leading 0 defined as required by the PoW.
2. The signature of the issuing node is valid.
3. It passes parents age checks. TODO:link timestamp specs



## Payloads
Payloads can contain arbitrary data up to `MAX_PAYLOAD_SIZE`, which allows building additional protocols on top of the base protocol in the same way as TCP/IP allows to define additional protocols on top of its generic data segment.

Payloads can recursively contain other payloads, which enables the creation of higher level protocols based on the same concepts of layers, as in traditional software and network architecture.

Payloads other than transactions are always liked with level of knowledge 3.

### User-defined payloads
A node can choose to interpret user-defined payloads by listenting to its specific **payload type** (possibly via third-party code/software). If a node does not know a certain **payload type**, it simply treats it as arbitrary data.

### Core payloads
The core protocol defines a number of payloads that every node needs to interpret and process in order to participate in the network.

- **Transactions:** Value transfers that constitute the ledger state.
- **Data:**  Pure data payloads allow to send unsigned messages.
- **dRNG:** Messages that contain randomness or committee declarations.
- **FPC:** Opinions on conflicts of transactions and timestamps of the messages, mainly issued by high mana nodes.


## Solidification
Due to the asynchronicity of the network, we may receive messages for which their past cone is not known yet. We refer to these messages as unsolid messages. It is not possible neither to approve nor to gossip unsolid messages. The actions required to obtain such missing messages is called solidification.
**Solidification** is the process of requesting missing referenced messages. It may be recursively repeated until all of a message's past cone up to the genesis (or snapshot) becomes solid.

In that way, the Tangle enables all nodes to retrieve all of a message's history, even the ones joining the network at a point later in time.

### Definitions
* **valid**: A message is considered valid if it passes the following filters from the solidifier and from the message booker:
    * solidifier: it checks if parents are valid,
    * booker: it check if the contained transaction is valid. Notice that only messages containing a transaction are required to perform this check..
> TODO: link to message layout semantic check, and transaction syntactic/semantic check in ledgerstate spec.
* **parents age check**: A check that ensures the timestamps of parents and child are valid, following the details defined in Timestamp specification [Parent age check](#parent-age-check)
> TODO: put the link of timestamp check
* **solid**: A message is solid if it passes parents age check and all its parents are stored in the storage, solid and valid.

### Detailed Design
During solidification, if a node is missing a referenced message, the corresponding message ID is stored in the `solidification buffer`. A node asks its neighbors for the missing message by sending a `solidification request` containing the message ID. Once the requested message is received from its neighbors, its message ID shall be removed from the `solidification buffer`. The requested message is marked as solid after it passes the standard solidification checks. If any of the checks fails, the message remains unsolid.

If a message gets solid, it shall walk through the rest of the data flow, then propagate the solid status to its future cone by performing the solidification checks on each of the messages in its future cone again.

![GoShimmer-flow-solidification_spec](https://user-images.githubusercontent.com/11289354/117009286-28333200-ad1e-11eb-8d0d-186c8d8ce373.png)


## Orphanage & Approval Switch
The Tangle builds approval of a given message by directly or indirectly attaching other messages in its future cone. Due to different reasons, such as the TSA not picking up a given message before its timestamp is still *fresh* or because its past cone has been rejected, a message can become orphan. This implies that the message cannot be included in the Tangle history since all the recent tips do not contain it in their past cone and thus, it cannot be retrieved during solidification. As a result, it might happen that honest messages and transactions would need to be reissued or reattached.
To overcome this limitation, we propose the `approval switch`. The idea is to minimize honest messages along with transactions getting orphaned, by assigning a different meaning to the parents of a message.

### Detailed design
Each message can express two levels of approval with respect to its parents:
* **Strong**: it defines approval for both the referenced message along with its entire past cone.
* **Weak**: it defines approval for the referenced message but not for its past cone.

Let's consider the following example:

![](https://i.imgur.com/rfpnkcg.png)

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

### Branch management
A message inherits the branch of its strong parents, while it does not inherit the branch of its weak parents.

#### Approval weight
The approval weight of a given message takes into account all of its future cone built over all its strong approvers.
Let's consider the following example:

![](https://i.imgur.com/a9FTyyg.png)

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

Therefore, we introduce approval weight (AW) (TODO: link) to measure the finality of any given message. Similarly to Bitcoin's 6 block rule, AW describes how deeply buried a message in the Tangle is. If a message reaches >50% of active consensus mana approving it, i.e., its future cone contains messages of nodes that together assert >50% of active consensus mana, it as finalized and, thus, confirmed. Specifically, in GoShimmer we use [markers](003-markers.md) to optimize AW calculations and approximate AW instead of tracking it for each message individually.
