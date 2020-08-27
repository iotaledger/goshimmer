# FPC Payload

## Motivation
The FPC protocol requires nodes to directly query randomly selected nodes for conflict resolution. However, the information produced during such a voting mechanism is not stored in the Tangle, rather only lives within the node's local metadata. This can be a problem for nodes joining the network at a later stage, specifically when a conflict is considered marked as level of knowledge 3 by the majority of the network, a new node cannot query it anymore. 
Moreover, when the quorum to query is randomly formed proportionally to mana, the highest mana nodes would need to reply to too many queries, as their probability to be included in the quorum of each node is high. 
We propose an optimization of the protocol that, in turn, should solve both of the above issues. The idea is to let each node free to choose whether writing its opinion on a given conflict and a given FPC round on the Tangle. 

## Detailed Design

### Payload
We need to first define the FPC Payload:

```
type Payload struct {
     Statements []Statement
}
```

```
type Statement struct {
    ID string
    Opinions []bool
}
```

### Registry

We also define an Opinion Registry where nodes can store and keep track of the opinions from each node after parsing FPC Payloads.

```
type Conflicts map[string][]Opinions
```

```
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


