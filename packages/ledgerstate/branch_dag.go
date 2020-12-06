package ledgerstate

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/stack"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions about the ledger state that exist in the
// tangle.
type BranchDAG struct {
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

	if cachedMasterBranch, stored := newBranchDAG.branchStorage.StoreIfAbsent(NewConflictBranch(NewBranchID(TransactionID{1}), nil, nil)); stored {
		cachedMasterBranch.Release()
	}

	return
}

// CreateConflictBranch retrieves the ConflictBranch that corresponds to the given details. It automatically creates and
// updates the ConflictBranch according to the new details if necessary.
func (b *BranchDAG) CreateConflictBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (cachedBranch *CachedBranch, newBranchCreated bool, err error) {
	normalizedParentBranchIDs, err := b.normalizeBranches(parentBranchIDs.Slice()...)
	if err != nil {
		err = xerrors.Errorf("failed to normalize parent Branches: %w", err)
		return
	}

	// create or load the branch
	cachedBranch = &CachedBranch{
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
	branch := cachedBranch.Unwrap()
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
			if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID)); stored {
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

func (b *BranchDAG) CreateAggregatedBranch(branchIDs ...BranchID) (cachedAggregatedBranch *CachedBranch, err error) {
	normalizedBranches, err := b.normalizeBranches(branchIDs...)
	if err != nil {
		err = xerrors.Errorf("failed to normalize Branches: %w", err)
		return
	}

	aggregatedBranch := NewAggregatedBranch(normalizedBranches)
	cachedAggregatedBranch = &CachedBranch{CachedObject: b.branchStorage.ComputeIfAbsent(aggregatedBranch.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		return aggregatedBranch
	})}
	return
}

// Branch retrieves the Branch with the given BranchID from the object storage.
func (b *BranchDAG) Branch(branchID BranchID) (cachedBranch *CachedBranch) {
	return &CachedBranch{CachedObject: b.branchStorage.Load(branchID.Bytes())}
}

// Shutdown shuts down the BranchDAG and persists its state.
func (b *BranchDAG) Shutdown() {
	b.shutdownOnce.Do(func() {
		b.branchStorage.Shutdown()
		b.childBranchStorage.Shutdown()
	})
}

// resolveConflictBranchIDs is an internal utility function that returns the BranchIDs of the ConflictBranches that the
// given Branches represent by resolving AggregatedBranches to their corresponding ConflictBranches.
func (b *BranchDAG) resolveConflictBranchIDs(branches ...BranchID) (conflictBranches BranchIDs, err error) {
	// initialize return variable
	conflictBranches = make(BranchIDs)

	// iterate through parameters and collect the conflict branches
	seenBranches := set.New()
	for _, branchID := range branches {
		// abort if branch was processed already
		if !seenBranches.Add(branchID) {
			continue
		}

		// process branch or abort if it can not be found
		if !b.Branch(branchID).Consume(func(branch Branch) {
			switch branch.Type() {
			case ConflictBranchType:
				conflictBranches[branch.ID()] = types.Void
			case AggregatedBranchType:
				for parentBranchID := range branch.Parents() {
					conflictBranches[parentBranchID] = types.Void
				}
			}
		}) {
			err = xerrors.Errorf("failed to load branch with %s: %w", branchID, cerrors.ErrFatal)
			return
		}
	}

	return
}

// normalizeBranches is an internal utility function that takes a list of BranchIDs and returns the the BranchIDS of the
// most special ConflictBranches that the given BranchIDs represent. It returns an error if the Branches are conflicting
// or any other unforeseen error occurred.
func (b *BranchDAG) normalizeBranches(branches ...BranchID) (normalizedBranches BranchIDs, err error) {
	// retrieve conflict branches and abort if we faced an error
	conflictBranches, err := b.resolveConflictBranchIDs(branches...)
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
			err = xerrors.Errorf("failed to load branch with %s: %w", conflictBranchID, cerrors.ErrFatal)
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
			err = xerrors.Errorf("failed to load branch with %s: %w", parentBranchID, cerrors.ErrFatal)
			return
		}

		// abort if we faced an error
		if err != nil {
			return
		}
	}

	return
}

// registerConflictMember is an internal utility function that creates the ConflictMember references of a Branch belong
// to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
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

/*


var utxoDAG *tangle.Tangle

func (branchManager *BranchManager) InheritBranch(tx *transaction.Transaction) (BranchIDs, err error) {
	consumedBranches := make([]MappedValue, 0)
	tx.Inputs().ForEach(func(outputID transaction.OutputID) bool {
		utxoDAG.TransactionOutput(outputID).Consume(func(output *tangle.Output) {
			consumedBranches = append(consumedBranches, output.MappedValue())
		})

		return true
	})

	normalizedBranches, err := branchManager.normalizeBranches(consumedBranches...)
	if err != nil {
		return
	}

	if len(normalizedBranches) == 1 {
		// done
	}

	if tx.IsConflicting() {

	} else {

	}
}
*/
