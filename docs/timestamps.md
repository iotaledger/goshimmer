# Timestamps

## Motivation
In order to enable snapshotting based on time constraints rather than special messages in the Tangle (e.g., milestones), nodes need to share the same perception of time. Specifically, they need to have consensus on the *age of messages*. Therefore, messages contain a field `timestamp` which represents the creation time of the message and is signed by the issuing node. 

Having consensus on the creation time of messages enables not only partial ordering but also new applications that require certain guarantees regarding time. 

In this document we propose a mechanism achieving consensus on message timestamps by combining a synchronous and an asynchronous approach. While online nodes can leverage FPC to vote on timestamps, nodes that join the network at a later time use an approach similar to On Tangle Voting to determine valid timestamps. 

## Requirements
1. Nodes participating in the network need to share the same perception of time.
2. Consensus on timestamps.

## Dependencies
+ Local markers: they are used to optimize the calculation required to check whether a given message future cone is approved by a given mana threshold.
+ FPC: it's used to perform voting on timestamps.

## Parameters
- `D` gratuitous network delay ~5 minutes. We assume all messages are delivered within this time.
- `w` window ~30 minutes. Require w>2D
- `Delta` max difference in consecutive timestamps. Require Delta>w+D
- `tw` transaction timestamp window. Max difference between message and transaction timestamp.


## Clock synchronization
Nodes need to share the same perception of time to fulfill `Req_1`. Therefore, we propose that nodes synchronize their clock on startup and resynchronize periodically every `60min` to counter [drift](https://en.wikipedia.org/wiki/Clock_drift) of local clocks. Instead of changing a nodes' system clock we introduce an `offset` parameter to adjust for differences between *network time* and local time of a node. Initially, the [Network Time Protocol (NTP)](https://en.wikipedia.org/wiki/Network_Time_Protocol) ([Go implementation](https://github.com/beevik/ntp)) can be used to achieve this task. 

```go
var offset time.Duration

func FetchTimeOffset() {
    resp, err := ntp.Query("0.pool.ntp.org")
    if err != nil {
        handle(err)
    }
    offset = resp.ClockOffset
}

func SyncedTime() time.Time {
    return time.Now().Add(offset)
}
```

### Failure to (re)sync clock
We gracefully shut down the node if:
- initial synchronization of time fails
- resynchronization fails for more than `3` times 

## General rules
Before a message can obtain `eligibility` status, i.e., becoming a valid tip and therefore part of the Tangle, it needs to fulfill certain criteria regarding its timestamp.

### Age of parents
We need the tangle to grow forward: we do not want incoming messages to reference extremely old messages. If any new message can reference any message in the Tangle, then a node will need to keep all messages readily available, precluding snapshotting. Additionally, we want to enforce a partial order, i.e., parents need to be older than children.

```go
func IsAgeOfParentsValid() bool {
    // check that parents are not too old
    if message.timestamp-parent1.timestamp > Delta {
        return false
    }
    if message.timestamp-parent2.timestamp > Delta {
        return false
    }
    
    // check that parents are not too young
    if message.timestamp <= parent1.timestamp {
        return false
    }
    if message.timestamp <= parent2.timestamp {
        return false
    }
    
    return true
}
```

### Message timestamp vs transaction timestamp
Transactions contain a timestamp that is signed by the user when creating the transaction. It is thus different from the timestamp in the message which is created and signed by the node. We require 
```
message.timestamp-tw < transaction.timestamp < message.timestamp
```
where `tw` defines the maximum allowed difference between both timestamps.

If a node receives a transaction from a user with an invalid timestamp it does not create a message but discards the transaction with a corresponding error message to the user. To prevent a user's local clock differences causing issues the node should offer an API endpoint to retrieve its `SyncedTime` according to the network time. 

### Reattachments
Reattachments of a transaction are possible during the time window `tw`. Specifically, a transaction can be reattached in a new message as long as the condition `message.timestamp-tw < transaction.timestamp` is fulfilled. If for some reason a transaction is not *picked up* (even after reattchment) and thus being orphaned, the user needs to create a new transaction with a current timestamp. 

### Age of UTXO
Inputs to a transaction (unspent outputs) inherit their spent time from the transaction timestamp. Similarly, unspent outputs inherit their creation time from the transaction timestamp as well. For a transaction to be considered valid we require
```
inputs.timestamp < transaction.timestamp
```
In other words, all inputs to a transaction need to have a smaller timestamp than the transaction. In turn, all created unspent outputs will have a greater timestamp than all inputs.

## Consensus
The timestamp should define the time when the message was created and issued to the Tangle, and this must be enforced to some degree through voting. Specifically, nodes will vote on whether the timestamp was issued within `w` of current local time. This time window is large to account for the network delay. 
Clearly, in order to have a correct perception of the timestamp quality, **we assume the node is in sync** (see section [Not in Sync](#Not_in_Sync) otherwise).
Voting on timestamps should not occur for every messsage. Specifically, only for those that arrive around the border of the threshold +-`w`.

The initial opinion and level of knowledge are set according to the following rule:

```
If |arrivalTime-currenTime|<w 
    Then opinion <- LIKE
    Else opinion <- DISLIKE
If ||arrivalTime-currenTime|-w|<D 
    Then level <- 1
Else If ||arrivalTime-currenTime|-w|<2D
    Then level <- 2
Else level <- 3
```

![](https://i.imgur.com/uSAFr8z.png)

For example, lets set `w` and `D` to 30 minutes and 5 minutes respectively. Let's assume that the current time is 12:00:00 and we have to evaluate a new message with timestamp set at 11:59:00. Since |11:59:00-12:00:00| < 30 minutes, we will set the opinion to `LIKE`. Moreover, since ||11:59:00-12:00:00| - 30 minutes | is greater than 5 minutes, and also grater than 2*5 minutes, we will set the level of knowledge for this opinion to 3 (i.e., the supermajority of the network should already have the same opinion).

Lets consider now a new message with timestamp 11:30:10. Since |11:30:10-12:00:00| < 30 minutes we will set the opinion to `LIKE`. However, since ||11:30:10-12:00:00| - 30 minutes | is lower than 5 minutes, we will set the level of knowledge for this opinion to 1, meaning that this message timestamp will be object of voting. 

In general, timestamps with level of knowledge 1 will be input of FPC, that will eventually trigger the `finalized` event, after which we can set a message as eligible (or discard, depending on the outcome). If instead, the timestamp we are considering, has already level of knowledge >= 2, we do not need to vote. Either it is eligible (marked as liked) or marked as disliked.

### Modification of FPC
The current FPC implementation is tailored to only vote on conflicting transactions. To allow for voting on timestamps, we will need to specifiy the type of voting object in the `queryRequest` message, and consequently, in the `queryReply` message.

```protobuf
message QueryRequest {
    repeated string conflictID = 1;
    repeated string timestampID = 2;
}

message QueryReply {
    repeated int32 conflictOpinion = 1;
    repeated int32 timestampOpinion = 2;
}
```

The voter interface implementation should then change to:

```go
// Voter votes on hashes.
type Voter interface {
	// Vote submits the given ID for voting with its initial Opinion and objectType.
	Vote(id string, initOpn Opinion, t objectType) error
	// IntermediateOpinion gets intermediate Opinion about the given ID and objectType.
	IntermediateOpinion(id string, t objectType) (Opinion, error)
	// Events returns the Events instance of the given Voter.
	Events() Events
}
```

Finally, the OpinionRetriever function should be defined as:

```go
type OpinionRetriever func(id string, t objectType) vote.Opinion
```

### FPC Timestamp Payload
To reduce the overhead for high mana nodes that would need to reply to tons of queries since they will, with high probability, be in the quorum of any node, we propose to write their opinion, as a form of statement, in the Tangle.
Such a message will contain an FPC timestamp payload defined as:

```go
type TimestampPayload struct {
     Statements []Statement
}
```

```go
type Statement struct {
    ID string
    Opinions []bool
}
```

### Registry

We also define an Opinion Registry where nodes can store and keep track of the opinions from each node after parsing FPC Timestamp Payloads.

```go
type Registry struct {
    Entry map[NodeID]TimestampOpinions
}
```

```go
type TimestampOpinions map[string][]Opinions
```

Given a nodeID and a messageID, a node can check if it has the required opinion in its registry, and thus use that during its FPC round, or if not, send a traditional query to the node.

Such a Registry should also be stored on a db so that restarting a node does not erase its state. Moreover, the Registry can be pruned accordingly to the snapshot.

### Broadcasting an FPC Timestamp Payload
A node, after forming its opinion for 1 or more timestamps during an FPC round, can prepare an FPC Timestamp Payload containing the result of that round and issue it on the Tangle.
The TSA must ensure that the FPC Timestamp Payload will be attached such that its probability of getting orphaned is low (or at least inversely proportional to its mana, once mana is integrated).

### Mana considerations

With mana integrated into FPC, both the decision to parse an FPC Timestamp Payload and thus, its storage on a node Registry, will be based on a parameter `MIN_MANA` defined by the node operator. Such a threshold should allow the node, to only use this FPC optimization for nodes that will have a high probability of being selected as part of its local FPC quorum.

### Not in Sync
Any node not in sync will have a wrong perception about the quality of timestamps. Thus, the idea is to not actively participating in any voting until its status is in sync.
Moreover, if a timestamp has been marked as `disliked` by the network, the message would get orphaned and the syncing node would not receive it from honest neighbors.
In general, a node that just completed the syncing phase should check, for each message, how much mana is in its future cone and set the opinion accordingly.

More specifically:

1. Run the solidification up to being in sync (by following beacons)
2. Derive local markers
3. Decide elegibility for every message (5-10% mana min threshold)


## Limitations
- The precision of the consensus of the timestamp is about +-30 minutes (depends on the parameter `w`).
- When not in sync, a different behaviour is required which complicates the protocol.
- Using NTP as clock synchronization mechanism as proposed is a single point of failure. It can only be considered as an initial implementation into GoShimmer and needs to be replaced by a decentralized alternative.