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
* **Epochs** divides timeline into intervals and keeps active consensus mana of each time slice, which also let us better define **active** and allow nodes to calculate approval weight based on the confirmed ledger state.
* **Markers** allows nodes to do approval weight approximation of a message by summing up the approval weight of its future markers. 

## Approval weight of messages


## Approval weight of branches

## Detailed design



