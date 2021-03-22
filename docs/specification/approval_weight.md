# Approval Weight
The goal of this document is to provide a high level overview of how approval weight will be implemented in GoShimmer.

## Introduction
**Approval weight** is the percentage of **active consensus mana** approving a message within the time interval of its issuing time. It is a metric for:
1. determining the grade of finality (level 2 and 3) of a message,
2. determining the majority’s perception of the ledger state when it comes to reorg.

The **active** consensus mana means taking into account only the "recently" active nodes, such that no very old or inactive nodes keep power over the consensus.

## Dependency
* Epoch
* marker

## Approval weight calculation
To calculate the approval weight of a message/branch, we first sum up the issuers' consensus mana of its future cone/supporters, then finally get the percentage of consensus mana approving it. However, to precisely calculate the approval weight of a message/branch has following difficulties:
1. it is costly to perform such calculation for a message if the network keeps growing,
2. the newer part of the Tangle may not be confirmed yet among nodes,
3. which node's consensus mana should be taken into account? (i.e., the definition of **active** nodes) 

To overcome these difficulties, we come up with **epochs** and **markers**. 
* **Epochs** divides timeline into intervals and keeps active consensus mana of each time slice, which also let us better define **active** and allow nodes to calculate approval weight based on the confirmed ledger state. For more details, refer to [Epoch Spec]().
* **Markers** allows nodes to do approval weight approximation of a message by summing up the approval weight of its future markers. For more details, refer to [Markers Spec](https://github.com/iotaledger/goshimmer/blob/develop/docs/specification/003-markers.md)

## Approval weight of markers
The approval weight of markers is calculated as follow:

1. Retrieve approvers of the marker within the same epoch
2. Retrieve the supporters of the branch where the message is.
3. Get the intersection of the previous points. 
4. Retrieve a list of active nodes along with their consensus mana of `oracleEpoch` (current epoch -2)
5. The approval weight of the marker is the sum of consensus mana of nodes from point 3.

### Approval weight approximation of messages
To approximate the approval weight of a message, we simply retrieve the approval weight of its future marker (`FM`) list. Since the message is in the past cone of its `FM`s, the approval weight and the finality will be at least the same as its `FM`s. If the markers are set frequently enough, it should be a good approximation.

After we have the approval weight of a message, it is marked **confirmed** when:
1. its branch is confirmed
2. the total approval weight is greater than a given threshold

## Approval weight of branches
The approval weight of branches is calculated as follow:

1. Get a list of supporters of a branch (i.e., a list of nodes that attach messages in that branch).
2. Get the `oracleEpoch` of the branch, which is the  `oracleEpoch` of the oldest epoch of the conflict set.
3. Sum the active consensus mana over all of its supporters.

After we have the approval weight of a branch, it is marked **confirmed** when:
1. its parents are confirmed,
2. its approval weight is greater than a given threshold.

**Note**: Once a branch gets confirmed, the conflicting ones get "lost."

## Detailed design



