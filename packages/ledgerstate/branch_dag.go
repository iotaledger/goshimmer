package ledgerstate

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
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

	return
}

// ConflictBranch fetches the ConflictBranch that corresponds to the given details. It automatically creates and updates
// the ConflictBranch according to the new details if necessary.
func (b *BranchDAG) ConflictBranch(conflictingTransactionID TransactionID, conflictingInputs []OutputID, parentBranches BranchIDs) (cachedBranch *CachedBranch, newBranchCreated bool) {
	// convert transaction models into branch models
	branchID := NewBranchID(conflictingTransactionID)
	conflictIDsSlice := make([]ConflictID, len(conflictingInputs))
	for i, conflictingInput := range conflictingInputs {
		conflictIDsSlice[i] = NewConflictID(conflictingInput)
	}
	conflictIDs := NewConflictIDs(conflictIDsSlice...)

	// create or load the branch
	cachedBranch = &CachedBranch{
		CachedObject: b.branchStorage.ComputeIfAbsent(branchID.Bytes(),
			func(key []byte) objectstorage.StorableObject {
				newBranch := NewConflictBranch(branchID, parentBranches, conflictIDs)
				newBranch.Persist()
				newBranch.SetModified()

				newBranchCreated = true

				return newBranch
			},
		),
	}
	branch := cachedBranch.Unwrap()
	if branch == nil {
		panic(fmt.Sprintf("failed to load branch with %s", branchID))
	}

	// type cast to ConflictBranch
	conflictBranch, typeCastOK := branch.(*ConflictBranch)
	if !typeCastOK {
		panic(fmt.Sprintf("failed to type cast Branch with %s to ConflictBranch", branchID))
	}

	// register references
	switch true {
	case newBranchCreated:
		// store child references
		for parentBranchID := range parentBranches {
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

// Branch retrieves the Branch with the given BranchID from the object storage.
func (b *BranchDAG) Branch(branchID BranchID) *CachedBranch {
	return &CachedBranch{CachedObject: b.branchStorage.Load(branchID.Bytes())}
}

// ConflictBranches returns a unique list of conflict branches that the given BranchIDs represent by resolving
// AggregatedBranches to their corresponding ConflictBranches.
func (b *BranchDAG) ConflictBranches(branches ...BranchID) (conflictBranches BranchIDs, err error) {
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

// Shutdown shuts down the BranchDAG and persists its state.
func (b *BranchDAG) Shutdown() {
	b.shutdownOnce.Do(func() {
		b.branchStorage.Shutdown()
		b.childBranchStorage.Shutdown()
	})
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
// NormalizeBranches checks if the branches are conflicting and removes superfluous ancestors.
func (branchManager *BranchManager) NormalizeBranches(branches ...MappedValue) (normalizedBranches BranchIDs, err error) {
	// retrieve conflict branches and abort if we either faced an error or are done
	conflictBranches, err := branchManager.ConflictBranches(branches...)
	if err != nil || len(conflictBranches) == 1 {
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
	checkConflictsAndQueueParents := func(currentBranch *ConflictBranch) {
		// abort if branch was traversed already
		if !traversedBranches.AddMapping(currentBranch.ID()) {
			return
		}

		// return error if conflict set was seen twice
		for conflictSetID := range currentBranch.Conflicts() {
			if !seenConflictSets.AddMapping(conflictSetID) {
				err = errors.New("branches conflicting")

				return
			}
		}

		// queue parents to be checked when traversing ancestors
		for _, parentBranchID := range currentBranch.Parents() {
			parentsToCheck.Push(parentBranchID)
		}

		return
	}

	// create normalized branch candidates (check their conflicts and queue parent checks)
	normalizedBranches = make(BranchIDs)
	for conflictBranchID := range conflictBranches {
		// add branch to the candidates of normalized branches
		normalizedBranches[conflictBranchID] = types.Void

		// check branch, queue parents and abort if we faced an error
		if !branchManager.ConflictBranch(conflictBranchID).Consume(checkConflictsAndQueueParents) {
			err = fmt.Errorf("error loading branch %v: %w", conflictBranchID, ErrBranchNotFound)
		}
		if err != nil {
			return
		}
	}

	// remove ancestors from the candidates
	for !parentsToCheck.IsEmpty() {
		// retrieve parent branch ID from stack
		parentBranchID := parentsToCheck.Pop().(MappedValue)

		// remove ancestor from normalized candidates
		delete(normalizedBranches, parentBranchID)

		// check branch, queue parents and abort if we faced an error
		if !branchManager.ConflictBranch(parentBranchID).Consume(checkConflictsAndQueueParents) {
			err = fmt.Errorf("error loading branch %v: %w", parentBranchID, ErrBranchNotFound)
		}
		if err != nil {
			return
		}
	}

	return
}

var utxoDAG *tangle.Tangle

func (branchManager *BranchManager) InheritBranch(tx *transaction.Transaction) (BranchIDs, err error) {
	consumedBranches := make([]MappedValue, 0)
	tx.Inputs().ForEach(func(outputID transaction.OutputID) bool {
		utxoDAG.TransactionOutput(outputID).Consume(func(output *tangle.Output) {
			consumedBranches = append(consumedBranches, output.MappedValue())
		})

		return true
	})

	normalizedBranches, err := branchManager.NormalizeBranches(consumedBranches...)
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
