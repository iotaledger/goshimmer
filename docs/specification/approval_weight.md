# Approval Weight
The goal of this document is to provide a high level overview of how approval weight will be implemented in GoShimmer.

## Introduction
**Approval weight** is a percentage of **active consensus mana** approving a message within a time interval of its issuing time. It is a metric for:
1. determining the grade of finality (level 2 and 3) of a message,
2. determining the majority’s perception of the ledger state when it comes to reorg.

The **active** consensus mana means taking into account only the **recently** active nodes, such that no very old or inactive nodes keep power over the consensus.

## Definitions
* **branch supporter**: A branch's supporter is a **node** that attach messages to that branch, meaning the node likes the branch.
* **marker/message supporter**: A marker/message's supporter is a **node** that attach messages that directly/inderictly reference it, including its issuer.

## Dependency
* Markers
* Epochs

## Approval weight calculation
To calculate the approval weight of a message/branch, we first sum up the issuers' consensus mana of its future cone/supporters, then finally get the percentage of consensus mana approving it. However, to precisely calculate the approval weight of a message/branch has following difficulties:
1. it is costly to perform such calculation for a message if the network keeps growing,
2. which node's consensus mana should be taken into account? (i.e., the definition of **active nodes**) 
3. the newer part of the Tangle may not be confirmed yet among nodes.

To overcome these difficulties, we come up with **Markers** and **Epochs** . 
* **Markers** allows nodes to approximate approval weight of a message by summing up the approval weight of its future markers. For more details of how Markers works, refer to [Markers Spec](http://goshimmer.docs.iota.org/specification/003-markers.html)
* **Epochs** divides timeline into intervals and keeps active consensus mana information of each time slice. With Epochs, difficulty 2 and 3 are solved:
    * an **active node** is a node that issues at least 1 message during a time slot.
    * nodes calculate approval weight based on the confirmed ledger state (the previous 2 epoch).

    For more details of how Epochs works, refer to [Epochs Spec](https://github.com/iotaledger/goshimmer/blob/docs/epochs/docs/002-epochs.md).

### Approval weight of markers
The approval weight of markers is calculated as follow:

1. Retrieve supporters of the marker within the same epoch
2. Retrieve the supporters of the branch where the message is.
3. Get the intersection of the previous points. 
4. Retrieve a list of active nodes along with their consensus mana of `oracleEpoch` (`currentEpoch -2`)
5. The approval weight of the marker is the sum of consensus mana of nodes from point 3.

### Approval weight approximation of messages
To approximate the approval weight of a message, we simply retrieve the approval weight of its future markers (`FM`s). Since the message is in the past cone of its `FM`s, the approval weight and the finality will be at least the same as its `FM`s. If the markers are set frequently enough, it should be a good approximation.

After we have the approval weight of a message, it is marked **confirmed** when the following 2 conditions meet:
1. its branch is confirmed
2. the total approval weight is greater than a given threshold

### Approval weight of branches
The approval weight of branches is calculated as follow:

1. Get a list of supporters of a branch (i.e., a list of nodes that attach messages in that branch).
2. Get the `oracleEpoch` of the branch, which is the  `oracleEpoch` of the oldest epoch of the conflict set.
3. Sum the active consensus mana over all of its supporters.

After we have the approval weight of a branch, it is marked **confirmed** when the following 2 conditions meet:
1. its parents are confirmed,
2. its approval weight is greater than a given threshold.

**Note**: Once a branch gets confirmed, the conflicting ones get "lost."

## Detailed design
In this section, we will describe implementation details of how approval weight are managed with Epochs and Markers, which includes:
1. approval weight management,
2. branch supporters management,
3. marker supporters management.

## Approval Weight Manager
In approval weight manager, the approval weight calculation described in [Approval weight calculation](#approval-weight-calculation) is performed. It contains:
* **epochs manager**: retrieve active consensus mana of the given epoch
* **supporters manager**: manage branch/marker supporters, refer to [Branch supporter / marker supporter management](#branch-supporter--marker-supporter-management).

## Branch supporter / marker supporter management
`SupporterManager` manages both branch and marker supporters. The following shows the details of essential data type and structure: `Supporter`, `Supporters` and `SupporterManager`.

### Supporter
A `Supporter` in the code is a node ID, and `Supporters` is a struct containing a set of `Supporter`. Both the data type and structure will be further used in `SupporterManager`.

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

`supporterManager` processes every message in order to update the branch supporters and those for markers:
```go
func (s *SupporterManager) ProcessMessage(messageID MessageID) {
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
        // branch supporter update
		s.updateBranchSupporters(message)
        // marker supporter update
		s.updateSequenceSupporters(message)
	})
}
```

### Branch Supporter management
`lastStatements` keeps the sequence number and the attached branch of the newest message that an issuer sent. The `supporterManager` will only continue if both conditions meet:
1. the statement of the issuer is new, meaning the sequence number is larger than the `lastStatement`.
2. the issuer attaches to the different branch from last message.

This check help nodes update supporters when it's necessary.

If the above conditions fulfill, `supporterManager` starts:
1. add the issuer as supporters to the branch, and propagate to its parent branches,
2. remove the issuer from the supporters of conflict branches, and propagate to its child branches.

Here's an example of how the propagation will look like:
![ApprovalWeight](https://user-images.githubusercontent.com/11289354/112409357-518e9480-8d54-11eb-8a40-19f4ab33ea35.png)


The green node issued **message 1** and attached it to `Branch 1.1 + Branch 4.1.1`. Thus, green node is a supporter of `Branch 1.1 + Branch 4.1.1`, and it's also a supporter to parent branches, which are (from top to bottom) `Branch 4.1.1`, `Branch 1.1`, `Branch 4.1`, `Branch 1`, and `Branch 4`.

Then, the green node issued **message 2** and attached it to `Branch 4.1.2`. This makes the green node a supporter of `Branch 4.1.2`, however, `Branch 4.1.1` is its conflict branch that makes green node not a supporter of `Branch 4.1.1`. 

`Branch 4.1`, `Branch 4` are parent branches of `Branch 4.1.2`, green node is still their supporters. Since `Branch 1.1`, `Branch 1` are not conflicting to either of `Branch 4.1.2`'s parents, the green node remains their supporter. 

Finally, green nodes issued **message 3**, which is in `Branch 2`. Now the green node is a supporter of `Branch 2`, and no longer a supporter of `Branch 1`, since `Branch 1` is conflicting to `Branch 2`. Note that, this supporter removal will propagate to child branches. Thus, green node is removed from `Branch 1.1`. 

`Branch 3`, `4` and both of their child branches have nothing to do with this attachement, the supporter status remains. 

#### `addSupportToBranch`
The codes below shows how a supporter is propagated to branches. For each processing branch, its conflicting branches (if have any) will trigger `revokeSupportFromBranch` to remove the supporter.
```go
func (s *SupporterManager) addSupportToBranch(branchID ledgerstate.BranchID, issuingTime time.Time, supporter Supporter, walk *walker.Walker) {
	// prepare supporters set for a new branch
    if _, exists := s.branchSupporters[branchID]; !exists {
		s.branchSupporters[branchID] = NewSupporters()
	}
    // abort if the supporter exists
	if !s.branchSupporters[branchID].Add(supporter) {
		return
	}

    // remove the supporter from the conflicting branches if it exists
	s.tangle.LedgerState.BranchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
		revokeWalker := walker.New()
		revokeWalker.Push(conflictingBranchID)
        // revoke supporters needs propagated to child branches
		for revokeWalker.HasNext() {
			s.revokeSupportFromBranch(revokeWalker.Next().(ledgerstate.BranchID), issuingTime, supporter, revokeWalker)
		}
	})

	s.Events.BranchSupportAdded.Trigger(branchID, issuingTime, supporter)

    // add parent branches to the walker for supporter propagation 
	s.tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		for parentBranchID := range branch.Parents() {
			walk.Push(parentBranchID)
		}
	})
}
```

#### `revokeSupportFromBranch`
The codes below shows how a supporter is revoked from a branch and its child branches. 
```go
func (s *SupporterManager) revokeSupportFromBranch(branchID ledgerstate.BranchID, issuingTime time.Time, supporter Supporter, walker *walker.Walker) {
	// check if the supporter supports the branch
    if supporters, exists := s.branchSupporters[branchID]; !exists || !supporters.Delete(supporter) {
		return
	}

	s.Events.BranchSupportRemoved.Trigger(branchID, issuingTime, supporter)

    // propagate supporter removal to child branches
	s.tangle.LedgerState.BranchDAG.ChildBranches(branchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
		if childBranch.ChildBranchType() != ledgerstate.ConflictBranchType {
			return
		}

		walker.Push(childBranch.ChildBranchID())
	})
}
```

### Markers supporter management 
Instead of keeping a list of supporters for each marker and collecting them by walking the Tangle, we keep a list of supporters along with its approved marker rank for each marker sequence. This approach provides a simple and fast look-up for marker/message supporters.

For each marker sequence, we keep a map of supporter to marker index, meaning a supporter supports a marker index `i`. This give the implication that the supporter supports all markers with index `<= i`.

Take the figure below as an example:
![MarkersApprovalWeight SequenceSupporters-Page-2](https://user-images.githubusercontent.com/11289354/112416694-21012780-8d61-11eb-8089-cb9f5b236f30.png)

The purple circles represent markers of the same sequence, the given numbers are marker indices. 

Four nodes (A to D) issue messages with Past Markers of the purple sequence. Node A and D issue messages having Past Marker with index 6, thus node A and D are the supporters of marker 6 that implicate they support all markers before, which is 1 to 5. On the other hand, node B issues a message having Past Marker with index 3, which implicates node B is a supporter for marker 1 and 2 as well.

This is a fast look-up and avoids walking through a marker's future cone when it comes to retreiving supporters for approval weight calculation. 

For example, to find all supporter of marker 2, we iterate through the map and filter out those support marker with `index >= 2`. In this case, all nodes are its supporters. As for marker 5, it has supporters node A and D, which fulfill the check: `index >= 5`.

Here is another more complicated example with parent references:
![MarkersApprovalWeight SequenceSupporters-Page-2(1)](https://user-images.githubusercontent.com/11289354/112433680-8cf18900-8d7d-11eb-8944-54030581a033.png)

The supporter will be propagated to the parent references sequence.

Node A issues message A2 having Past Markers `[1,4], [3,5]`, which implicates node A is a supporter for marker `[1,1]` to `[1,4]`, `[2,1]` to `[2,3]`, and `[3,4], [3,5]` as well.

#### Sequence supporter
Each marker sequence has a corresponding `SequenceSupporters` struct to keep marker supporters information.

<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>sequenceID</td>
        <td>markers.SequenceID</td>
        <td>The sequence ID of sequence that holds this struct.</td>
    </tr>
    <tr>
        <td>supportersPerIndex</td>
        <td>map[Supporter]markers.Index</td>
        <td>Map supporters to the supported marker index of this sequence.</td>
    </tr>
    <tr>
        <td>supportersPerIndexMutex</td>
        <td>sync.RWMutex</td>
        <td>A read/write mutex to protect read/write operations.</td>
    </tr>
</table> 

`SequenceSupporters` has the following methods for management:
* `NewSequenceSupporters(sequenceID markers.SequenceID) (sequenceSupporters *SequenceSupporters)`: Create a `SequenceSupporters` for the given sequence ID.
* `AddSupporter(supporter Supporter, index markers.Index) (added bool)`: Add a new Supporter of a given index to the Sequence.
* `Supporters(index markers.Index) (supporters *Supporters)`: Returns a list of supporters for the given marker index.
* `SequenceID() (sequenceID markers.SequenceID)`: Return the SequenceID that is being tracked.

#### `addSupportToMarker`
The codes below shows how a supporter is propagated within marker sequences.
```go
func (s *SupporterManager) addSupportToMarker(marker *markers.Marker, issuingTime time.Time, supporter Supporter, walker *walker.Walker) {
	// create a SequenceSupporters if the sequence doesn't have one
    s.tangle.Storage.SequenceSupporters(marker.SequenceID(), func() *SequenceSupporters {
		return NewSequenceSupporters(marker.SequenceID())
	}).Consume(func(sequenceSupporters *SequenceSupporters) {
        // add the supporter to the sequence
		if sequenceSupporters.AddSupporter(supporter, marker.Index()) {
			s.Events.SequenceSupportUpdated.Trigger(marker, issuingTime, supporter)

            // propagate the supporter to parent marker sequences
			s.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
				sequence.ParentReferences().HighestReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
					walker.Push(markers.NewMarker(sequenceID, index))

					return true
				})
			})
		}
	})
}
```

