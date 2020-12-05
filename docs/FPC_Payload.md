# FPC Statement

## Motivation
The FPC protocol requires nodes to directly query randomly selected nodes for conflict resolution. However, the information produced during such a voting mechanism is not stored in the Tangle, rather only lives within the node's local metadata. This can be a problem for nodes joining the network at a later stage, specifically when a conflict is considered marked as level of knowledge 3 by the majority of the network, a new node cannot query it anymore. 
Moreover, when the quorum to query is randomly formed proportionally to mana, the highest mana nodes would need to reply to too many queries, as their probability to be included in the quorum of each node is high. 
We propose an optimization of the protocol that, in turn, should solve both of the above issues. The idea is to let each node be free to choose whether writing its opinion on a given conflict and a given FPC round on the Tangle. 

## Detailed Design

### Payload
We need to first define the FPC Statement payload:

```go
type Statement struct {
    ConflictsCount  uint32
	Conflicts       Conflicts
	TimestampsCount uint32
	Timestamps      Timestamps
}

type Conflict struct {
	ID transaction.ID
	Opinion
}

type Timestamp struct {
	ID tangle.MessageID
	Opinion
}
```

### Registry

We also define an Opinion Registry where nodes can store and keep track of the opinions from each node after parsing FPC Statements.


```go
type Registry struct {
    nodesView map[identity.ID]*View
}

type View struct {
	NodeID     identity.ID
	Conflicts  map[transaction.ID]Opinions
	Timestamps map[tangle.MessageID]Opinions
}
```

Given a nodeID and a ConflictID (or a messageID for timestamps), a node can check if it has the required opinion in its registry, and thus use that during its FPC round, or if not, send a traditional query to the node.

Moreover, the Registry can be pruned accordingly to the snapshot.

### Broadcasting an FPC Statement
A node, after forming its opinion for 1 or more conflicts during an FPC round, can prepare an FPC statement containing the result of that round and issue it on the Tangle.
The TSA must ensure that the FPC statement will be attached such that its probability of getting orphan is low (or at least inversely proportional to its mana, once mana is integrated).

## Mana considerations

With mana integrated into FPC, both the decision to parse an FPC statement and thus, its storage on a node Registry, will be based on a parameter `MIN_MANA` defined by the node operator. Such a threshold should allow the node, to only use this FPC optimization for nodes that will have a high probability of being selected as part of its local FPC quorum.

## Optimiaztion
+ use the outputID to define the conflict set: this allows to compress all the conflicting transactions into one element (i.e., the outputID) and only reference the like one (if any).
+ leverage monotonicity: this allows to use the property so that only a subset of conflcting transactions are going to be necessary to describe the opinion of the whole conflict set.

## Further Questions
+ How many statements should we fit in one FPC statement? An adversary could inflate the number of conflicts. 
+ Should high mana nodes write statements even if their conflict set is empty? Let's say every 10-30s. This would also benefit Confirmation Confidence and ability to sync.
+ What should the mana threshold be to write your statement? Should the frequency of the empty statements be proportional to mana?

