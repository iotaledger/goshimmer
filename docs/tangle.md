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
- `MIN_STRONG_PARENTS=1` The minimum amount of strong parents a message needs to have.

## General concept
![Tangle](https://i.ibb.co/RyqbZzN/tangle.png)

The Tangle is an immutable data structure that consists out of **messages** that **reference previous messages** via their crypthograpic hashes, creating a directed acyclic graph (DAG) of messages. Every participant in the network keeps track of its *local Tangle* and derives its *ledger state* from it. 

### Terminology
- **Genesis**: The genesis message is used to bootstrap the Tangle. It is the first message and does not have parents. It is marked as solid.
- **Past cone**: All messages that are directly or indirectly referenced by a message are called its past cone
- **Future cone**: All messages that directly or indirectly reference a message are called its future cone.
- **Solidity**: A message is marked as solid if its entire past cone until the Genesis (or the latest snapshot) is known.
- **Parents**: A message directly references between 2-8 previous messages that we call its **parents**. A parent can be either **strong** or **weak** [TO DO: LINK TO THE APPROVAL SWITCH SPEC].
- **Approvers**: Parents are approved by their referencing messages called **approvers**. It is thus a reverse mapping of parents. As in the parents definition, an approver might be either **strong** or **weak**.

## Messages
Messages are created and signed by nodes. Besides several fields of metadata they carry a **payload**. The maximum message size is `MAX_MESSAGE_SIZE`.

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
        <td>The amount of parents proceeding.</td>
    </tr>
    <tr>
        <td>Parents type</td>
        <td>uint8</td>
        <td>Bitwise encoding of parent type matching the order of proceeding parents starting at <code>least significant bit</code>. <code>1</code> indicates a strong parent, while <code>0</code> signals a weak parent. At least <code>MIN_STRONG_PARENTS</code> parent type needs to be strong.</td>
    </tr>
    <tr>
        <td colspan="1">
            Parents <code>between(2,8)</code>
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
                        <td>The Message ID of the <i>Message</i> it references.</td>
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
        <td>The length of the Payload. Since its type may be unknown to the node it must be declared in advance. 0 length means no payload will be attached.</td>
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
                            The type of the payload. It will instruct the node how to parse the fields that follow. Types in the range of 0-127 are "core types" that all nodes are expected to know.
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
2. When we are done parsing the message, there is not any trailing bytes left that were not parsed.
4. At least 1 and at most 8 distinct parents are given, ordered ASC and at least `MIN_STRONG_PARENTS` are strong parents. 

### Semantic Validation
Messages that do no pass the Semantic Validation are discarded. Only semantically valid messages continue in the data flow, i.e., pass to the eligibility check.

A message is semantically valid if:
- The Message PoW Hash contains at least the number of leading 0 defined as required by the adaptive PoW algo [TO DO: LINK TO THE APOW SPEC].
- The signature from the issuing node is valid.

### Eligibility check
If a message gets to this point (i.e., if a message passed the semantic validation), it will be scheduled, it will have its payload processed and will be added to the local Tangle of a node. Nevertheless, only eligible messages might become available for tip selection in the future. Eligibility of a message **does not** express any opinion about the attachment location of a message. It solely evaluates a message according to its timestamp. Notice that, the eligibility of a message does not imply anything about the payload of the message; a message containing a disliked payload can still be eligible, even though this message will be never included in the weak/strong tip set. Thus, the TSA chooses from a subset of the eligible messages.

A message is an eligible message if, after passing the syntactical and semantical validation:
- It is solid
- It has a level 2 or 3 good timestamp (TODO: link timestamp spec)
- It passes parents age checks (TODO: link timestamp spec)
- Its parents are eligible




### Metadata
Next to a message itself, a node needs to store additional data that describe its local perception of this message and are not part of the Tangle.

<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>receivedTime</td>
        <td>time</td>
        <td>The local time the message was received by the node.</td>
    </tr>
    <tr>
        <td>solid</td>
        <td>bool</td>
        <td>Denotes whether a message is solid, i.e., its past cone is known.</td>
    </tr>
    <tr>
        <td>solidificationTime</td>
        <td>time</td>
        <td>The local time the message got solid.</td>
    </tr>
    <tr>
        <td>opinion</td>
        <td>Opinion</td>
        <td>Contains a nodes' opinion on the timestamp of a message. A triple <code>(opinion, level, timeFormed)</code>, where opinion is a bool, level is in the set {1,2,3}, and timeFormed is a time. The opinionField is also manipulated by FPC. TODO: this should be defined probably in the FPC specs?</td>
    </tr>
    <tr>
        <td>eligible</td>
        <td>bool</td>
        <td>Denotes whether a message is eligible.</td>
    </tr>
</table>


## Payloads
Payloads can contain arbitrary data up to `MAX_PAYLOAD_SIZE` that can be defined by the user and that allow to build additional protocols on top of the base protocol in the same way as TCP/IP allows to define additional protocols on top of its generic data segment.

Payloads can recursively contain other payloads which enables the creation of higher level protocols based on the same concepts of layers as in traditional software and network architecture.

Payloads other than transactions are always liked with level of knowledge 3. 

### User-defined payloads
A node can choose to interpret user-defined payloads by listenting to its specific **payload type** (possibly via third-party code/software). If a node does not know a certain **payload type** it is simply treated as arbitrary data.

### Core payloads
The core protocol defines a number of payloads that every node needs to interpret and process in order to participate in the network.

- **Transactions:** Value transfers that constitue the ledger state. 
- **dRNG:** Messages that contain randomness or committee declarations.
- **FPC:** Opinions on conflicts, mainly issued by high mana nodes.

    
## Solidification
A message is said to be **solid** on a node when all its parents are known to the node and also marked as solid.

If a node is missing a referenced message it is stored in the **solidification buffer** and not yet processed. A node can ask its neighbors for the missing message by sending a **solidification request** containing the message hash. This process can be recursively repeated until all of a message's past cone until the genesis (or snapshot) become solid which is known as **solidification**. In that way the Tangle enables even nodes joining the network at a point later in time to retrieve all of a message's history.

### Naive approach
Approach:
1. Send solidification request for message immediately to all neighbors
2. If not received after `solidificationRetryInterval` seconds, send solidification request again. Repeat until received.

This approach simply requests missing messages recursivley until all of them become solid. While easy to implement it has some drawbacks. Each  parent that needs to be requested adds another RTT and message complexity (solidification request * neighbors). 

Optimizations:
- do not send solidification request immediately: messages can arrive out of order but should generally arrive within a small time window since other nodes only gossip messages on solidification
- instead of asking for messages one by one a node could ask for messages of a certain timeframe (e.g. its local snapshot time until now)
- send solidification request only to a subset of neighbors

### Possible attacks
A malicious node can send unsolidifiable messages to a node. A simple protection is to only repeat solidification requests `solidificationMaxRepeat` times before the message gets deleted from the node's solidification buffer. 


## Orphanage
Messages that are considered to be not eligible are ignored during tip selection. This process of leaving undesired messages (or more in general, whenever a message is not being approved) behind is called **orphaning messages**. It is important that nodes share the same perception on which messages should be orphaned. 

Orphaned messages can be safely deleted during snapshotting and are not visible to nodes that later join the network because they are not reachable when requesting the missing messages from the tips during solidification.

## Finality
Users need to know whether their information will not be orphaned. However, finality is inherently probabilistic. For instance, consider the following scenario. An attacker can trivially maintain a chain of messages that do not approve any other message. At any given point in time, it is possible that all messages will be orphaned except this chain. This is incredibly unlikely, but yet still possible.

We introduce several grades of finality. The higher the grade of finality, the less likely it is to be orphaned.

- **Grade 1:** when it becomes eligible & its payload is liked with level of knowledge >=2
- **Grade 2:** when it reaches a given confidence level	> 5-10% active consensus mana
- **Grade 3:** when it reaches an even bigger confidence level > 50% active consensus mana
