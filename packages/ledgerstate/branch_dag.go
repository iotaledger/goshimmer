package ledgerstate

import (
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

type BranchDAG struct {
	branchStorage *objectstorage.ObjectStorage
}

func NewBranchDAG(store kvstore.KVStore) (newBranchDAG *BranchDAG) {
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	newBranchDAG = &BranchDAG{
		branchStorage: osFactory.New(PrefixBranchStorage, BranchFromObjectStorage, objectStorageOptions...),
	}

	return
}

func (b *BranchDAG) RegisterConflict(conflictingTransactionID TransactionID, conflictingInputs []OutputID, parentBranches BranchIDs) (cachedBranch *CachedBranch, newBranchCreated bool) {
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

	// create the referenced entities and references
	cachedBranch.Retain().Consume(func(branch Branch) {
		// store references from the parent branches to this new child branch (only once when the branch is created
		// since updating the parents happens through ElevateConflictBranch and is only valid for conflict Branches)
		if newBranchCreated {
			for _, parentBranchID := range parentBranches {
				if cachedChildBranch, stored := b.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID)); stored {
					cachedChildBranch.Release()
				}
			}
		}
	})

	return
}

func (b *BranchDAG) Branch(branchID BranchID) *CachedBranch {
	return &CachedBranch{CachedObject: b.branchStorage.Load(branchID.Bytes())}
}

// ConflictBranches returns a unique list of conflict branches that the given branches represent.
// It resolves the aggregated branches to their corresponding conflict branches.
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
