# Approval Weight
The goal of this document is to provide a high level overview of how approval weight will be implemented in GoShimmer.

## Introduction
**Approval weight** is the percentage of **active consensus mana** approving a message within the time interval of its issuing time. It is a metric for:
1. determining the grade of finality (level 2 and 3) of a message,
2. determining the majority’s perception of the ledger state when it comes to reorg.

The **active** consensus mana means taking into account only the "recently" active nodes, such that no very old or inactive nodes keep power over the consensus.

## Definitions
* **branch supporter**: A branch's approver is a **node** that attach messages to that branch, meaning the node likes the branch.
* **marker/message approver**: A marker/message's approver is a **node** that attach messages that directly/inderictly reference it, including its issuer.

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
In this section, we will describe how approval weight are managed with epoch and markers, which includes:
1. supporters of a branch as well as the approvers of markers management,
2. approval weight management.

## Branch supporter / message approver management
Instead of keeping a list of approvers for each marker and collecting supporters of a branch by walking the Tangle, we keep the list of approvers along with its approved marker rank for each branch.

This approach gives us benefits:
1. easy to get the supporters of a branch,
2. simple and fast look-up of marker approver look-up,
3. easy to maintain.

### Supporter
A `Supporter` in the code is a node ID, and `Supporters` is a struct containing a set of `Supporter`. Both the data type and structure will be further used in supporter manager.

<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>Supporter</td>
        <td>identity.ID</td>
        <td>A supporter of a branch.</td>
    </tr>
    <tr>
        <td>Supporters</td>
        <td>set.Set</td>
        <td>A list of supporters of a branch.</td>
    </tr>
</table> 

`Supporters` has following methods:
* `Add(supporter Supporter) (added bool)`: Add the `Supporter` from the Set and returns true if it did added.
* `Delete(supporter Supporter) (deleted bool)`: Removes the `Supporter` from the Set and returns true if it did exist.
* `Has(supporter Supporter) (has bool)`: Returns true if the Supporter exists in the Set.
* `ForEach(callback func(supporter Supporter))`: Iterates through the `Supporters` and calls the callback for every element.

### SupporterManager
The `supporterManager` holds the following fields: 
<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>Events</td>
        <td>*SupporterManagerEvents</td>
        <td>A list of defined events for supporter manager.</td>
    </tr>
    <tr>
        <td>tangle</td>
        <td>*Tangle</td>
        <td>The Tangle.</td>
    </tr>
    <tr>        
        <td>lastStatements</td>
        <td colspan="1">map[Supporter]*Statement            
            <details open="true">
            <table>                
                <summary>Statement</summary>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>SequenceNumber</td>
                        <td>uint64</td>
                        <td>The sequence number of the statement message.</td>
                    </tr>
                    <tr>
                        <td>BranchID</td>
                        <td>ledgerstate.BranchID</td>
                        <td>The Branch ID of the branch it supports.</td>
                    </tr>
            </table>
            </details>
        </td>        
        <td>A map of supporter to `Statement` of its latest issued message.</td>
    </tr>
    <tr>
        <td>branchSupporters</td>
        <td>map[ledgerstate.BranchID]*Supporters</td>
        <td>A map of branch ID to its supporters.</td>
    </tr>
</table> 







