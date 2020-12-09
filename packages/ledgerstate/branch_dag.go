package ledgerstate

import (
	"container/list"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/stack"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions about the ledger state that exist in the
// tangle.
type BranchDAG struct {
	// Events is a container for all of the BranchDAG related events.
	Events                *BranchDAGEvents
	branchStorage         *objectstorage.ObjectStorage
	childBranchStorage    *objectstorage.ObjectStorage
	conflictStorage       *objectstorage.ObjectStorage
	conflictMemberStorage *objectstorage.ObjectStorage
	shutdownOnce          sync.Once
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(store kvstore.KVStore) (newBranchDAG *BranchDAG) {
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	newBranchDAG = &BranchDAG{
		branchStorage:         osFactory.New(PrefixBranchStorage, BranchFromObjectStorage, branchStorageOptions...),
		childBranchStorage:    osFactory.New(PrefixChildBranchStorage, ChildBranchFromObjectStorage, childBranchStorageOptions...),
		conflictStorage:       osFactory.New(PrefixConflictStorage, ConflictFromObjectStorage, conflictStorageOptions...),
		conflictMemberStorage: osFactory.New(PrefixConflictMemberStorage, ConflictMemberFromObjectStorage, conflictMemberStorageOptions...),
	}
	newBranchDAG.init()

	return
}

// RetrieveConflictBranch retrieves the ConflictBranch that corresponds to the given details. It automatically creates
// and updates the ConflictBranch according to the new details if necessary.
func (b *BranchDAG) RetrieveConflictBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedConflictBranch *CachedBranch, newBranchCreated bool, err error) {
	normalizedParentBranchIDs, err := b.normalizeBranches(parentBranchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to normalize parent Branches: %w", err)
		return
	}

	// create or load the branch
	cachedConflictBranch = &CachedBranch{
		CachedObject: b.branchStorage.ComputeIfAbsent(branchID.Bytes(),
			func(key []byte) objectstorage.StorableObject {
				newBranch := NewConflictBranch(branchID, normalizedParentBranchIDs, conflictIDs)
				newBranch.Persist()
				newBranch.SetModified()

				newBranchCreated = true

				return newBranch
			},
		),
	}
	branch := cachedConflictBranch.Unwrap()
	if branch == nil {
		err = xerrors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
		return
	}

	// type cast to ConflictBranch
	conflictBranch, typeCastOK := branch.(*ConflictBranch)
	if !typeCastOK {
		err = xerrors.Errorf("failed to type cast Branch with %s to ConflictBranch: %w", branchID, cerrors.ErrFatal)
		return
	}

	// register references
	switch true {
	case newBranchCreated:
		// store child references
		for parentBranchID := range normalizedParentBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID, ConflictBranchType)); stored {
				cachedChildBranch.Release()
			}
		}

		// store ConflictMember references
		for conflictID := range conflictIDs {
			b.registerConflictMember(conflictID, branchID)
		}
	default:
		// store new ConflictMember references
		for conflictID := range conflictIDs {
			if conflictBranch.AddConflict(conflictID) {
				b.registerConflictMember(conflictID, branchID)
			}
		}
	}

	return
}

// RetrieveAggregatedBranch retrieves the AggregatedBranch that corresponds to the given BranchIDs. It automatically
// creates the AggregatedBranch if it didn't exist, yet.
func (b *BranchDAG) RetrieveAggregatedBranch(parentBranchIDs BranchIDs) (cachedAggregatedBranch *CachedBranch, newBranchCreated bool, err error) {
	normalizedParentBranchIDs, err := b.normalizeBranches(parentBranchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to normalize Branches: %w", err)
		return
	}

	aggregatedBranch := NewAggregatedBranch(normalizedParentBranchIDs)
	cachedAggregatedBranch = &CachedBranch{CachedObject: b.branchStorage.ComputeIfAbsent(aggregatedBranch.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		newBranchCreated = true

		return aggregatedBranch
	})}

	if newBranchCreated {
		for parentBranchID := range normalizedParentBranchIDs {
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, aggregatedBranch.ID(), AggregatedBranchType)); stored {
				cachedChildBranch.Release()
			}
		}
	}

	return
}

// SetBranchPreferred sets the preferred flag of the given Branch. It returns true if the value has been updated or an
// error if it failed.
func (b *BranchDAG) SetBranchPreferred(branchID BranchID, preferred bool) (modified bool, err error) {
	return b.setBranchPreferred(b.Branch(branchID), preferred)
}

// Branch retrieves the Branch with the given BranchID from the object storage.
func (b *BranchDAG) Branch(branchID BranchID) (cachedBranch *CachedBranch) {
	return &CachedBranch{CachedObject: b.branchStorage.Load(branchID.Bytes())}
}

// ChildBranches loads the references to the ChildBranches of the given Branch from the object storage.
func (b *BranchDAG) ChildBranches(branchID BranchID) (cachedChildBranches CachedChildBranches) {
	cachedChildBranches = make(CachedChildBranches, 0)
	b.childBranchStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedChildBranches = append(cachedChildBranches, &CachedChildBranch{CachedObject: cachedObject})

		return true
	}, branchID.Bytes())

	return
}

// Conflict loads a Conflict from the object storage.
func (b *BranchDAG) Conflict(conflictID ConflictID) *CachedConflict {
	return &CachedConflict{CachedObject: b.conflictStorage.Load(conflictID.Bytes())}
}

// ConflictMembers loads the referenced ConflictMembers of a Conflict from the object storage.
func (b *BranchDAG) ConflictMembers(conflictID ConflictID) (cachedConflictMembers CachedConflictMembers) {
	cachedConflictMembers = make(CachedConflictMembers, 0)
	b.conflictMemberStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedConflictMembers = append(cachedConflictMembers, &CachedConflictMember{CachedObject: cachedObject})

		return true
	}, conflictID.Bytes())

	return
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (b *BranchDAG) Prune() (err error) {
	for _, storage := range []*objectstorage.ObjectStorage{
		b.branchStorage,
		b.childBranchStorage,
		b.conflictStorage,
		b.conflictMemberStorage,
	} {
		if err = storage.Prune(); err != nil {
			err = xerrors.Errorf("failed to prune the object storage (%v): %w", err, cerrors.ErrFatal)
			return
		}
	}

	b.init()

	return
}

// Shutdown shuts down the BranchDAG and persists its state.
func (b *BranchDAG) Shutdown() {
	b.shutdownOnce.Do(func() {
		b.branchStorage.Shutdown()
		b.childBranchStorage.Shutdown()
		b.conflictStorage.Shutdown()
		b.conflictMemberStorage.Shutdown()
	})
}

// init is an internal utility function that initializes the BranchDAG by creating the root of the DAG (MasterBranch).
func (b *BranchDAG) init() {
	cachedMasterBranch, stored := b.branchStorage.StoreIfAbsent(NewConflictBranch(NewBranchID(TransactionID{1}), nil, nil))
	if !stored {
		return
	}

	(&CachedBranch{CachedObject: cachedMasterBranch}).Consume(func(branch Branch) {
		branch.SetPreferred(true)
		branch.SetLiked(true)
		branch.SetFinalized(true)
		branch.SetConfirmed(true)
	})
}

// resolveConflictBranchIDs is an internal utility function that returns the BranchIDs of the ConflictBranches that the
// given Branches represent by resolving AggregatedBranches to their corresponding ConflictBranches.
func (b *BranchDAG) resolveConflictBranchIDs(branchIDs BranchIDs) (conflictBranchIDs BranchIDs, err error) {
	// initialize return variable
	conflictBranchIDs = make(BranchIDs)

	// iterate through parameters and collect the conflict branches
	seenBranches := set.New()
	for branchID := range branchIDs {
		// abort if branch was processed already
		if !seenBranches.Add(branchID) {
			continue
		}

		// process branch or abort if it can not be found
		if !b.Branch(branchID).Consume(func(branch Branch) {
			switch branch.Type() {
			case ConflictBranchType:
				conflictBranchIDs[branch.ID()] = types.Void
			case AggregatedBranchType:
				for parentBranchID := range branch.Parents() {
					conflictBranchIDs[parentBranchID] = types.Void
				}
			}
		}) {
			err = xerrors.Errorf("failed to load Branch with %s: %w", branchID, cerrors.ErrFatal)
			return
		}
	}

	return
}

// normalizeBranches is an internal utility function that takes a list of BranchIDs and returns the BranchIDS of the
// most special ConflictBranches that the given BranchIDs represent. It returns an error if the Branches are conflicting
// or any other unforeseen error occurred.
func (b *BranchDAG) normalizeBranches(branchIDs BranchIDs) (normalizedBranches BranchIDs, err error) {
	// retrieve conflict branches and abort if we faced an error
	conflictBranches, err := b.resolveConflictBranchIDs(branchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to resolve ConflictBranchIDs: %w", err)
		return
	}

	// return if we are done
	if len(conflictBranches) == 1 {
		normalizedBranches = conflictBranches
		return
	}

	// return the master branch if the list of conflict branches is empty
	if len(conflictBranches) == 0 {
		return BranchIDs{MasterBranchID: types.Void}, nil
	}

	// introduce iteration variables
	traversedBranches := set.New()
	seenConflictSets := set.New()
	parentsToCheck := stack.New()

	// checks if branches are conflicting and queues parents to be checked
	checkConflictsAndQueueParents := func(currentBranch Branch) {
		currentConflictBranch, typeCastOK := currentBranch.(*ConflictBranch)
		if !typeCastOK {
			err = xerrors.Errorf("failed to type cast Branch with %s to ConflictBranch: %w", currentBranch.ID(), cerrors.ErrFatal)
			return
		}

		// abort if branch was traversed already
		if !traversedBranches.Add(currentConflictBranch.ID()) {
			return
		}

		// return error if conflict set was seen twice
		for conflictSetID := range currentConflictBranch.Conflicts() {
			if !seenConflictSets.Add(conflictSetID) {
				err = xerrors.Errorf("combined Branches are conflicting: %w", ErrInvalidStateTransition)
				return
			}
		}

		// queue parents to be checked when traversing ancestors
		for _, parentBranchID := range currentConflictBranch.Parents() {
			parentsToCheck.Push(parentBranchID)
		}
	}

	// create normalized branch candidates (check their conflicts and queue parent checks)
	normalizedBranches = make(BranchIDs)
	for conflictBranchID := range conflictBranches {
		// add branch to the candidates of normalized branches
		normalizedBranches[conflictBranchID] = types.Void

		// check branch and queue parents
		if !b.Branch(conflictBranchID).Consume(checkConflictsAndQueueParents) {
			err = xerrors.Errorf("failed to load Branch with %s: %w", conflictBranchID, cerrors.ErrFatal)
			return
		}

		// abort if we faced an error
		if err != nil {
			return
		}
	}

	// remove ancestors from the candidates
	for !parentsToCheck.IsEmpty() {
		// retrieve parent branch ID from stack
		parentBranchID := parentsToCheck.Pop().(BranchID)

		// remove ancestor from normalized candidates
		delete(normalizedBranches, parentBranchID)

		// check branch, queue parents and abort if we faced an error
		if !b.Branch(parentBranchID).Consume(checkConflictsAndQueueParents) {
			err = xerrors.Errorf("failed to load Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
			return
		}

		// abort if we faced an error
		if err != nil {
			return
		}
	}

	return
}

// registerConflictMember is an internal utility function that creates the ConflictMember references of a Branch
// belonging to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID BranchID) {
	(&CachedConflict{CachedObject: b.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) objectstorage.StorableObject {
		newConflict := NewConflict(conflictID)
		newConflict.Persist()
		newConflict.SetModified()

		return newConflict
	})}).Consume(func(conflict *Conflict) {
		if cachedConflictMember, stored := b.conflictMemberStorage.StoreIfAbsent(NewConflictMember(conflictID, branchID)); stored {
			conflict.IncreaseMemberCount()

			cachedConflictMember.Release()
		}
	})
}

// setBranchPreferred updates the preferred flag of the given Branch. It returns true if the value has been updated or
// an error if it failed.
func (b *BranchDAG) setBranchPreferred(cachedBranch *CachedBranch, preferred bool) (modified bool, err error) {
	// release the CachedBranch when we are done
	defer cachedBranch.Release()

	// unwrap ConflictBranch
	conflictBranch, err := cachedBranch.UnwrapConflictBranch()
	if err != nil {
		err = xerrors.Errorf("failed to load ConflictBranch with %s: %w", cachedBranch.ID(), cerrors.ErrFatal)
		return
	}

	// execute case dependent logic
	switch preferred {
	case true:
		// iterate through all Conflicts of the current Branch and set their ConflictMembers to be not preferred
		for conflictID := range conflictBranch.Conflicts() {
			// iterate through all ConflictMembers and set them to not preferred
			cachedConflictMembers := b.ConflictMembers(conflictID)
			for _, cachedConflictMember := range cachedConflictMembers {
				// unwrap the ConflictMember
				conflictMember := cachedConflictMember.Unwrap()
				if conflictMember == nil {
					cachedConflictMembers.Release()
					err = xerrors.Errorf("failed to load ConflictMember of %s: %w", conflictID, cerrors.ErrFatal)
					return
				}

				// skip the current Branch
				if conflictMember.BranchID() == conflictBranch.ID() {
					continue
				}

				// update the other ConflictMembers to be not preferred
				if _, err = b.setBranchPreferred(b.Branch(conflictMember.BranchID()), false); err != nil {
					cachedConflictMembers.Release()
					err = xerrors.Errorf("failed to propagate preferred changes to other ConflictMembers: %w", err)
					return
				}
			}
			cachedConflictMembers.Release()
		}

		// abort if the branch was preferred already
		if modified = conflictBranch.SetPreferred(true); !modified {
			return
		}

		// trigger event
		b.Events.BranchPreferred.Trigger(NewBranchDAGEvent(cachedBranch))

		// update the preferred status of the future cone (it only affect the AggregatedBranches)
		if err = b.updatePreferredStatusOfFutureCone(conflictBranch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update preferred status of future cone of Branch with %s: %w", conflictBranch.ID(), err)
			return
		}

		// update the liked status of the future cone (if necessary)
		if err = b.updateLikedStatusOfFutureCone(conflictBranch.ID(), true); err != nil {
			err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", conflictBranch.ID(), err)
			return
		}
	case false:
		// set the branch to be not preferred
		if modified = conflictBranch.SetPreferred(false); modified {
			// trigger event
			b.Events.BranchUnpreferred.Trigger(NewBranchDAGEvent(cachedBranch))

			// update the preferred status of the future cone (it only affect the AggregatedBranches)
			if propagationErr := b.updatePreferredStatusOfFutureCone(conflictBranch.ID(), false); propagationErr != nil {
				err = xerrors.Errorf("failed to propagate preferred changes to AggregatedBranches: %w", propagationErr)
				return
			}

			// update the liked status of the future cone (if necessary)
			if propagationErr := b.updateLikedStatusOfFutureCone(conflictBranch.ID(), false); propagationErr != nil {
				err = xerrors.Errorf("failed to update liked status of future cone of Branch with %s: %w", conflictBranch.ID(), propagationErr)
				return
			}
		}
	}

	return
}

// updateLikedStatusOfFutureCone is an internal utility function that checks if the given Branch has become liked and
// propagates the changes to the Branch itself and to its children (if necessary).
func (b *BranchDAG) updateLikedStatusOfFutureCone(branchID BranchID, liked bool) (err error) {
	// initialize stack for iteration
	branchStack := list.New()
	branchStack.PushBack(branchID)

	// iterate through stack
ProcessStack:
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentStackElement := branchStack.Front()
		branchStack.Remove(currentStackElement)

		// load Branch
		currentCachedBranch := b.Branch(currentStackElement.Value.(BranchID))

		// unwrap current CachedBranch
		currentBranch := currentCachedBranch.Unwrap()
		if currentBranch == nil {
			currentCachedBranch.Release()
			err = xerrors.Errorf("failed to load Branch with %s: %w", currentCachedBranch.ID(), cerrors.ErrFatal)
			return
		}

		// execute case dependent logic
		switch liked {
		case true:
			// abort if the current Branch is not preferred
			if !currentBranch.Preferred() {
				currentCachedBranch.Release()
				continue
			}

			// abort if any parent Branch is not liked
			for parentBranchID := range currentBranch.Parents() {
				// load parent Branch
				cachedParentBranch := b.Branch(parentBranchID)

				// unwrap parent Branch
				parentBranch := cachedParentBranch.Unwrap()
				if parentBranch == nil {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					err = xerrors.Errorf("failed to load parent Branch with %s: %w", parentBranchID, cerrors.ErrFatal)
					return
				}

				// abort if parent Branch is not liked
				if !parentBranch.Liked() {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					continue ProcessStack
				}

				// release parent CachedBranch
				cachedParentBranch.Release()
			}

			// abort if the Branch is already liked
			if !currentBranch.SetLiked(true) {
				currentCachedBranch.Release()
				continue
			}

			// trigger event
			b.Events.BranchLiked.Trigger(NewBranchDAGEvent(currentCachedBranch))
		case false:
			// abort if the current Branch is disliked already
			if !currentBranch.SetLiked(false) {
				currentCachedBranch.Release()
				continue
			}

			// trigger event
			b.Events.BranchDisliked.Trigger(NewBranchDAGEvent(currentCachedBranch))
		}

		// iterate through ChildBranch references and queue found Branches for propagation
		cachedChildBranchReferences := b.ChildBranches(currentBranch.ID())
		for _, cachedChildBranchReference := range cachedChildBranchReferences {
			// unwrap ChildBranch reference
			childBranchReference := cachedChildBranchReference.Unwrap()
			if childBranchReference == nil {
				currentCachedBranch.Release()
				cachedChildBranchReferences.Release()
				err = xerrors.Errorf("failed to load ChildBranch reference: %w", cerrors.ErrFatal)
				return
			}

			// queue child Branch for propagation
			branchStack.PushBack(childBranchReference.ChildBranchID())
		}
		cachedChildBranchReferences.Release()

		// release current CachedBranch
		currentCachedBranch.Release()
	}

	return
}

// updatePreferredStatusOfFutureCone updates the preferred flag of the AggregatedBranches that are direct children
// of the Branch whose preferred flag was updated.
func (b *BranchDAG) updatePreferredStatusOfFutureCone(branchID BranchID, preferred bool) (err error) {
	// initialize stack with children of type AggregatedBranch of the passed in Branch (we only update the preferred
	// status of AggregatedBranches as their status depends on their parents)
	branchStack := list.New()
	b.ChildBranches(branchID).Consume(func(childBranch *ChildBranch) {
		if childBranch.ChildBranchType() == AggregatedBranchType {
			branchStack.PushBack(childBranch.ChildBranchID())
		}
	})

	// iterate through stack
ProcessStack:
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentAggregatedBranchEntry := branchStack.Front()
		branchStack.Remove(currentAggregatedBranchEntry)

		// load Branch
		currentCachedBranch := b.Branch(currentAggregatedBranchEntry.Value.(BranchID))

		// unwrap current CachedBranch
		currentBranch, typeErr := currentCachedBranch.UnwrapAggregatedBranch()
		if typeErr != nil {
			currentCachedBranch.Release()
			err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", currentCachedBranch.ID(), typeErr)
			return
		}
		if currentBranch == nil {
			currentCachedBranch.Release()
			err = xerrors.Errorf("failed to load AggregatedBranch with %s: %w", currentCachedBranch.ID(), cerrors.ErrFatal)
			return
		}

		// execute case dependent logic
		switch preferred {
		case true:
			// only continue if all parents are preferred
			for aggregatedParentBranchID := range currentBranch.Parents() {
				// load parent Branch
				cachedParentBranch := b.Branch(aggregatedParentBranchID)

				// unwrap parent Branch
				parentBranch := cachedParentBranch.Unwrap()
				if parentBranch == nil {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					err = xerrors.Errorf("failed to load parent Branch with %s: %w", aggregatedParentBranchID, cerrors.ErrFatal)
					return
				}

				// abort if the parent Branch is not preferred
				if !parentBranch.Preferred() {
					currentCachedBranch.Release()
					cachedParentBranch.Release()
					continue ProcessStack
				}

				// release parent CachedBranch
				cachedParentBranch.Release()
			}

			// trigger event if the value was changed
			if currentBranch.SetPreferred(true) {
				b.Events.BranchPreferred.Trigger(NewBranchDAGEvent(currentCachedBranch))
			}
		case false:
			// trigger event if the value was changed
			if currentBranch.SetPreferred(false) {
				b.Events.BranchUnpreferred.Trigger(NewBranchDAGEvent(currentCachedBranch))
			}
		}

		// release current CachedBranch
		currentCachedBranch.Release()
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchDAGEvents //////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGEvents is a container for all of the BranchDAG related events.
type BranchDAGEvents struct {
	// BranchPreferred gets triggered whenever a Branch becomes preferred that was not preferred before.
	BranchPreferred *events.Event

	// BranchUnpreferred gets triggered whenever a Branch becomes unpreferred that was preferred before.
	BranchUnpreferred *events.Event

	// BranchLiked gets triggered whenever a Branch becomes liked that was not liked before.
	BranchLiked *events.Event

	// BranchLiked gets triggered whenever a Branch becomes preferred that was not preferred before.
	BranchDisliked *events.Event

	// BranchFinalized gets triggered when a decision on a Branch is finalized and there will be no further state
	// changes regarding its preferred state.
	BranchFinalized *events.Event

	// BranchConfirmed gets triggered whenever a Branch becomes confirmed that was not confirmed before.
	BranchConfirmed *events.Event

	// BranchRejected gets triggered whenever a Branch becomes rejected that was not rejected before.
	BranchRejected *events.Event
}

// NewBranchDAGEvents creates a container for all of the BranchDAG related events.
func NewBranchDAGEvents() *BranchDAGEvents {
	return &BranchDAGEvents{
		BranchPreferred:   events.NewEvent(branchEventCaller),
		BranchUnpreferred: events.NewEvent(branchEventCaller),
		BranchLiked:       events.NewEvent(branchEventCaller),
		BranchDisliked:    events.NewEvent(branchEventCaller),
		BranchFinalized:   events.NewEvent(branchEventCaller),
		BranchConfirmed:   events.NewEvent(branchEventCaller),
		BranchRejected:    events.NewEvent(branchEventCaller),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchDAGEvent ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAGEvent represents an event object that contains a Branch and that is triggered by the BranchDAG.
type BranchDAGEvent struct {
	Branch *CachedBranch
}

// NewBranchDAGEvent creates a new BranchDAGEvent from the given CachedBranch.
func NewBranchDAGEvent(branch *CachedBranch) (newBranchEvent *BranchDAGEvent) {
	return &BranchDAGEvent{Branch: branch}
}

// Retain marks all of the CachedObjects in the event to be retained for future use.
func (b *BranchDAGEvent) Retain() *BranchDAGEvent {
	b.Branch.Retain()

	return b
}

// Release marks all of the CachedObjects in the event as not used anymore.
func (b *BranchDAGEvent) Release() *BranchDAGEvent {
	b.Branch.Release()

	return b
}

// branchEventCaller is an internal utility function that type casts the generic parameters of the event handler to
// their specific type. It automatically retains all the contained CachedObjects for their use in the registered
// callback.
func branchEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(branch *BranchDAGEvent))(params[0].(*BranchDAGEvent).Retain())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
