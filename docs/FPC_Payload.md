# FPC Payload

## Motivation
The FPC protocol requires nodes to directly query randomly selected nodes for conflict resolution. However, the information produced during such a voting mechanism is not stored in the Tangle, rather only lives within the node's local metadata. This can be a problem for nodes joining the network at a later stage, specifically when a conflict is considered marked as level of knowledge 3 by the majority of the network, a new node cannot query it anymore. 
Moreover, when the quorum to query is randomly formed proportionally to mana, the highest mana nodes would need to reply to too many queries, as their probability to be included in the quorum of each node is high. 
We propose an optimization of the protocol that, in turn, should solve both of the above issues. The idea is to let each node be free to choose whether writing its opinion on a given conflict and a given FPC round on the Tangle. 

## Detailed Design

### Payload
We need to first define the FPC Payload:

```go
type Payload struct {
     Value Statements []Statement
}
```

```go
type Statement struct {
    ID string
    Opinions []bool
}
```

### Registry

We also define an Opinion Registry where nodes can store and keep track of the opinions from each node after parsing FPC Payloads.

```go
type Conflicts map[string][]Opinions
```

```go
type Registry struct {
    Entry map[NodeID]Conflicts
}
```

Given a nodeID and a ConflictID, a node can check if it has the required opinion in its registry, and thus use that during its FPC round, or if not, send a traditional query to the node.

Such a Registry should also be stored on a db so that restarting a node does not erase its state. Moreover, the Registry can be pruned accordingly to the snapshot.

### Broadcasting an FPC Payload
A node, after forming its opinion for 1 or more conflicts during an FPC round, can prepare an FPC Payload containing the result of that round and issue it on the Tangle.
The TSA must ensure that the FPC Payload will be attached such that its probability of getting orphan is low (or at least inversely proportional to its mana, once mana is integrated).

## Mana considerations

With mana integrated into FPC, both the decision to parse an FPC Payload and thus, its storage on a node Registry, will be based on a parameter `MIN_MANA` defined by the node operator. Such a threshold should allow the node, to only use this FPC optimization for nodes that will have a high probability of being selected as part of its local FPC quorum.


## Optimiaztion
+ use the outputID to define the conflict set: this allows to compress all the conflicting transactions into one element (i.e., the outputID) and only reference the like one (if any).
+ leverage monotonicity: this allows to use the property so that only a subset of conflcting transactions are going to be necessary to describe the opinion of the whole conflict set.


## Further Questions
+ How many statements should we fit in one FPC payload? An adversary could inflate the number of conflicts. 
+ Should high mana nodes write statements even if their conflict set is empty? Let's say every 10-30s. This would also benefit Confirmation Confidence and ability to sync.
+ What should the mana threshold be to write your statement? Should the frequency of the empty statements be proportional to mana?

# Timestamp

## Motivation
ToDO

## Notes
+ If we do not find any opinion about a target message timestamp, we have to look at the confidence level of that message to derive its timestamp final opinion.
+ Preconsensus via FPC Payloads/Statements
+ Check every hour the synchronization of your local clock

### FPC Timestamp Payload
We need to first define the FPC Timestamp Payload:

```go
type TimestampPayload struct {
     Statements []Statement
}
```

## Brainstorming design
+ timestamps with level of knowledge 1 will be input of FPC, that will eventually trigger the `finalized` event, after which we can set or not those message as eligible.
+ if the timestamp we are considering, has already level of knowledge >= 2, we do not need to vote. Either is eligible (marked as liked) or marked as disliked.
+ 

```protobuf
message QueryRequest {
    repeated string conflictID = 1;
    repeated string timestampID = 1;
}

message QueryReply {
    repeated int32 conflictOpinion = 1;
    repeated int32 timestampOpinion = 1;
}
```

## Open questions
+ Deal with wrong clocks (centralized VS decentralized approaches)

## Clock synchronization
+ Start with a plugin based on NTP (e.g., getCurrentTime), then change plugin with a decentralized one.
+ Define a function to get the synced current time without adjusting the system clock, but instead using an offset (e.g., getCorrectTime).

## Syncing
If it’s bad, then you should not receive it while syncying.
To be more sure, you just need to check how much mana is in its future cone.

1. We do the solidification up to being more or less in sync (we follow beacons)
2. We derive the checkpoints
3. You can then decide for every message their elegibility (5-10% mana min threshold)
