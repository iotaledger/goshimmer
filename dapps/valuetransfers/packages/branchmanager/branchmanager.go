package branchmanager

import (
	"container/list"
	"fmt"
	"sort"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

// BranchManager is an entity that manages the branches of a UTXODAG. It offers methods to add, delete and modify
// Branches. It automatically keeps track of the "monotonicity" of liked and disliked by propagating these flags
// according to the structure of the Branch-DAG.
type BranchManager struct {
	// stores the branches
	branchStorage *objectstorage.ObjectStorage

	// stores the references which branch is the child of which parent (we store this in a separate "reference entity"
	// instead of the branch itself, because there can potentially be a very large amount of child branches and we do
	// not want the branch instance to get bigger and bigger (it would slow down its marshaling/unmarshaling).
	childBranchStorage *objectstorage.ObjectStorage

	// stores the conflicts that create constraints regarding which branches can be aggregated.
	conflictStorage *objectstorage.ObjectStorage

	// stores the references which branch is part of which conflict (we store this in a separate "reference entity"
	// instead of the conflict itself, because there can be a very large amount of member branches and we do not want
	// the conflict instance to get bigger and bigger (it would slow down its marshaling/unmarshaling).
	conflictMemberStorage *objectstorage.ObjectStorage

	// contains the Events of the BranchManager
	Events *Events
}

// New is the constructor of the BranchManager.
func New(store kvstore.KVStore) (branchManager *BranchManager) {
	osFactory := objectstorage.NewFactory(store, storageprefix.ValueTransfers)

	branchManager = &BranchManager{
		branchStorage:         osFactory.New(osBranch, osBranchFactory, osBranchOptions...),
		childBranchStorage:    osFactory.New(osChildBranch, osChildBranchFactory, osChildBranchOptions...),
		conflictStorage:       osFactory.New(osConflict, osConflictFactory, osConflictOptions...),
		conflictMemberStorage: osFactory.New(osConflictMember, osConflictMemberFactory, osConflictMemberOptions...),
		Events: &Events{
			BranchPreferred:   events.NewEvent(branchCaller),
			BranchUnpreferred: events.NewEvent(branchCaller),
			BranchLiked:       events.NewEvent(branchCaller),
			BranchDisliked:    events.NewEvent(branchCaller),
			BranchFinalized:   events.NewEvent(branchCaller),
			BranchConfirmed:   events.NewEvent(branchCaller),
			BranchRejected:    events.NewEvent(branchCaller),
		},
	}
	branchManager.init()

	return
}

// Branch loads a Branch from the objectstorage.
func (branchManager *BranchManager) Branch(branchID BranchID) *CachedBranch {
	return &CachedBranch{CachedObject: branchManager.branchStorage.Load(branchID.Bytes())}
}

// ChildBranches loads the references to the ChildBranches of the given Branch.
func (branchManager *BranchManager) ChildBranches(branchID BranchID) CachedChildBranches {
	childBranches := make(CachedChildBranches, 0)
	branchManager.childBranchStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		childBranches = append(childBranches, &CachedChildBranch{CachedObject: cachedObject})

		return true
	}, branchID.Bytes())

	return childBranches
}

// Conflict loads a Conflict from the objectstorage.
func (branchManager *BranchManager) Conflict(conflictID ConflictID) *CachedConflict {
	return &CachedConflict{CachedObject: branchManager.conflictStorage.Load(conflictID.Bytes())}
}

// ConflictMembers loads the referenced members of a Conflict from the objectstorage.
func (branchManager *BranchManager) ConflictMembers(conflictID ConflictID) CachedConflictMembers {
	conflictMembers := make(CachedConflictMembers, 0)
	branchManager.conflictMemberStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		conflictMembers = append(conflictMembers, &CachedConflictMember{CachedObject: cachedObject})

		return true
	}, conflictID.Bytes())

	return conflictMembers
}

// Fork adds a new Branch to the branch-DAG and automatically creates the Conflicts and references if they don't exist.
// It can also be used to update an existing Branch and add it to additional conflicts.
func (branchManager *BranchManager) Fork(branchID BranchID, parentBranches []BranchID, conflicts []ConflictID) (cachedBranch *CachedBranch, newBranchCreated bool) {
	// create or load the branch
	cachedBranch = &CachedBranch{
		CachedObject: branchManager.branchStorage.ComputeIfAbsent(branchID.Bytes(),
			func(key []byte) objectstorage.StorableObject {
				newBranch := NewBranch(branchID, parentBranches)
				newBranch.Persist()
				newBranch.SetModified()

				newBranchCreated = true

				return newBranch
			},
		),
	}

	// create the referenced entities and references
	cachedBranch.Retain().Consume(func(branch *Branch) {
		// store references from the parent branches to this new child branch (only once when the branch is created
		// since updating the parents happens through ElevateConflictBranch and is only valid for conflict Branches)
		if newBranchCreated {
			for _, parentBranchID := range parentBranches {
				if cachedChildBranch, stored := branchManager.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID)); stored {
					cachedChildBranch.Release()
				}
			}
		}

		// store conflict + conflict references
		for _, conflictID := range conflicts {
			if branch.addConflict(conflictID) {
				(&CachedConflict{CachedObject: branchManager.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) objectstorage.StorableObject {
					newConflict := NewConflict(conflictID)
					newConflict.Persist()
					newConflict.SetModified()

					return newConflict
				})}).Consume(func(conflict *Conflict) {
					if cachedConflictMember, stored := branchManager.conflictMemberStorage.StoreIfAbsent(NewConflictMember(conflictID, branchID)); stored {
						conflict.IncreaseMemberCount()

						cachedConflictMember.Release()
					}
				})
			}
		}
	})

	return
}

// ElevateConflictBranch moves a branch to a new parent. This is necessary if a new conflict appears in the past cone
// of an already existing conflict.
func (branchManager *BranchManager) ElevateConflictBranch(branchToElevate BranchID, newParent BranchID) (isConflictBranch bool, modified bool, err error) {
	// load the branch
	currentCachedBranch := branchManager.Branch(branchToElevate)
	defer currentCachedBranch.Release()

	// abort if we could not load the branch
	currentBranch := currentCachedBranch.Unwrap()
	if currentBranch == nil {
		err = fmt.Errorf("failed to load branch '%s'", branchToElevate)

		return
	}

	// abort if this branch is aggregated (only conflict branches can be elevated)
	if currentBranch.IsAggregated() {
		return
	}
	isConflictBranch = true

	// remove old child branch references
	branchManager.childBranchStorage.Delete(marshalutil.New(BranchIDLength * 2).
		WriteBytes(currentBranch.ParentBranches()[0].Bytes()).
		WriteBytes(branchToElevate.Bytes()).
		Bytes(),
	)

	// add new child branch references
	if cachedChildBranch, stored := branchManager.childBranchStorage.StoreIfAbsent(NewChildBranch(newParent, branchToElevate)); stored {
		cachedChildBranch.Release()
	}

	// update parent of branch
	if modified, err = currentBranch.updateParentBranch(newParent); err != nil {
		return
	}

	return
}

// BranchesConflicting returns true if the given Branches are part of the same Conflicts and can therefore not be
// merged.
func (branchManager *BranchManager) BranchesConflicting(branchIds ...BranchID) (branchesConflicting bool, err error) {
	// iterate through branches and collect conflicting branches
	traversedBranches := make(map[BranchID]types.Empty)
	blacklistedBranches := make(map[BranchID]types.Empty)
	for _, branchID := range branchIds {
		// add the current branch to the stack of branches to check
		ancestorStack := list.New()
		ancestorStack.PushBack(branchID)

		// iterate over all ancestors and collect the conflicting branches
		for ancestorStack.Len() >= 1 {
			// retrieve branch from stack
			firstElement := ancestorStack.Front()
			currentBranchID := firstElement.Value.(BranchID)
			ancestorStack.Remove(firstElement)

			// abort if we have seen this branch already
			if _, traversedAlready := traversedBranches[currentBranchID]; traversedAlready {
				continue
			}

			// abort if this branch was blacklisted by another branch already
			if _, branchesConflicting = blacklistedBranches[currentBranchID]; branchesConflicting {
				return
			}

			// unpack the branch and abort if we failed to load it
			currentCachedBranch := branchManager.Branch(currentBranchID)
			currentBranch := currentCachedBranch.Unwrap()
			if currentBranch == nil {
				err = fmt.Errorf("failed to load branch '%s'", currentBranchID)

				currentCachedBranch.Release()

				return
			}

			// add the parents of the current branch to the list of branches to check
			for _, parentBranchID := range currentBranch.ParentBranches() {
				ancestorStack.PushBack(parentBranchID)
			}

			// abort the following checks if the branch is aggregated (aggregated branches have no own conflicts)
			if currentBranch.IsAggregated() {
				currentCachedBranch.Release()

				continue
			}

			// iterate through the conflicts and take note of its member branches
			for conflictID := range currentBranch.Conflicts() {
				for _, cachedConflictMember := range branchManager.ConflictMembers(conflictID) {
					// unwrap the current ConflictMember
					conflictMember := cachedConflictMember.Unwrap()
					if conflictMember == nil {
						cachedConflictMember.Release()

						continue
					}

					if conflictMember.BranchID() == currentBranchID {
						cachedConflictMember.Release()

						continue
					}

					// abort if this branch was found as a conflict of another branch already
					if _, branchesConflicting = traversedBranches[conflictMember.BranchID()]; branchesConflicting {
						cachedConflictMember.Release()
						currentCachedBranch.Release()

						return
					}

					// store the current conflict in the list of seen conflicting branches
					blacklistedBranches[conflictMember.BranchID()] = types.Void

					cachedConflictMember.Release()
				}
			}

			currentCachedBranch.Release()

			traversedBranches[currentBranchID] = types.Void
		}
	}

	return
}

// AggregateBranches takes a list of BranchIDs and tries to "aggregate" the given IDs into a new Branch. It is used to
// correctly "inherit" the referenced parent Branches into a new one.
func (branchManager *BranchManager) AggregateBranches(branches ...BranchID) (cachedAggregatedBranch *CachedBranch, err error) {
	// return the MasterBranch if we have no branches in the parameters
	if len(branches) == 0 {
		cachedAggregatedBranch = branchManager.Branch(MasterBranchID)

		return
	}

	// return the first branch if there is only one
	if len(branches) == 1 {
		cachedAggregatedBranch = branchManager.Branch(branches[0])

		return
	}

	// check if the branches are conflicting
	branchesConflicting, err := branchManager.BranchesConflicting(branches...)
	if err != nil {
		return
	}
	if branchesConflicting {
		err = fmt.Errorf("the given branches are conflicting and can not be aggregated")

		return
	}

	// filter out duplicates and shared ancestor Branches (abort if we faced an error)
	deepestCommonAncestors, err := branchManager.findDeepestCommonDescendants(branches...)
	if err != nil {
		return
	}

	// if there is only one branch that we found, then we are done
	if len(deepestCommonAncestors) == 1 {
		for _, firstBranchInList := range deepestCommonAncestors {
			cachedAggregatedBranch = firstBranchInList
		}

		return
	}

	// if there is more than one parent: aggregate
	aggregatedBranchID, aggregatedBranchParents, err := branchManager.determineAggregatedBranchDetails(deepestCommonAncestors)
	if err != nil {
		return
	}

	// create or update the aggregated branch (the conflicts is an empty list because aggregated branches are not
	// directly conflicting with other branches but are only used to propagate votes to all of their parents)
	cachedAggregatedBranch, _ = branchManager.Fork(aggregatedBranchID, aggregatedBranchParents, []ConflictID{})

	return
}

// SetBranchPreferred is the method that allows us to modify the preferred flag of a branch.
func (branchManager *BranchManager) SetBranchPreferred(branchID BranchID, preferred bool) (modified bool, err error) {
	return branchManager.setBranchPreferred(branchManager.Branch(branchID), preferred)
}

// SetBranchLiked is the method that allows us to modify the liked flag of a branch (it propagates to the parents).
func (branchManager *BranchManager) SetBranchLiked(branchID BranchID, liked bool) (modified bool, err error) {
	return branchManager.setBranchLiked(branchManager.Branch(branchID), liked)
}

// SetBranchFinalized modifies the finalized flag of a branch. It automatically triggers
func (branchManager *BranchManager) SetBranchFinalized(branchID BranchID) (modified bool, err error) {
	return branchManager.setBranchFinalized(branchManager.Branch(branchID))
}

// GenerateAggregatedBranchID generates an aggregated BranchID from the handed in BranchIDs.
func (branchManager *BranchManager) GenerateAggregatedBranchID(branchIDs ...BranchID) BranchID {
	sort.Slice(branchIDs, func(i, j int) bool {
		for k := 0; k < len(branchIDs[k]); k++ {
			if branchIDs[i][k] < branchIDs[j][k] {
				return true
			} else if branchIDs[i][k] > branchIDs[j][k] {
				return false
			}
		}

		return false
	})

	marshalUtil := marshalutil.New(BranchIDLength * len(branchIDs))
	for _, branchID := range branchIDs {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return blake2b.Sum256(marshalUtil.Bytes())
}

func (branchManager *BranchManager) setBranchFinalized(cachedBranch *CachedBranch) (modified bool, err error) {
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = fmt.Errorf("failed to unwrap branch")

		return
	}

	if modified = branch.setFinalized(true); !modified {
		return
	}

	branchManager.Events.BranchFinalized.Trigger(cachedBranch)

	// propagate finalized to aggregated child branches
	if err = branchManager.propagateFinalizedToAggregatedChildBranches(cachedBranch.Retain()); err != nil {
		return
	}

	if !branch.Preferred() {
		branchManager.propagateRejectedToChildBranches(cachedBranch.Retain())

		return
	}

	// update all other branches that are in the same conflict sets to be not preferred and also finalized
	for conflictID := range branch.Conflicts() {
		branchManager.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			// skip the branch which just got preferred
			if conflictMember.BranchID() == branch.ID() {
				return
			}

			_, err = branchManager.setBranchPreferred(branchManager.Branch(conflictMember.BranchID()), false)
			if err != nil {
				return
			}
			_, err = branchManager.setBranchFinalized(branchManager.Branch(conflictMember.BranchID()))
			if err != nil {
				return
			}
		})
	}

	// schedule confirmed checks of children
	err = branchManager.propagateConfirmedToChildBranches(cachedBranch.Retain())

	return
}

// propagateFinalizedToAggregatedChildBranches propagates the finalized flag to the aggregated child branches of the
// given branch. An aggregated branch is finalized if all of its parents are finalized.
func (branchManager *BranchManager) propagateFinalizedToAggregatedChildBranches(cachedBranch *CachedBranch) (err error) {
	// initialize stack with the child branches of the given branch
	propagationStack := list.New()
	cachedBranch.Consume(func(branch *Branch) {
		branchManager.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
			propagationStack.PushBack(branchManager.Branch(childBranch.ChildID()))
		})
	})

	// iterate through stack to propagate the changes to child branches
	for propagationStack.Len() >= 1 {
		stackElement := propagationStack.Front()
		stackElement.Value.(*CachedBranch).Consume(func(branch *Branch) {
			// abort if the branch is not aggregated
			if !branch.IsAggregated() {
				return
			}

			// abort if not all parents are confirmed
			for _, parentBranchID := range branch.ParentBranches() {
				cachedParentBranch := branchManager.Branch(parentBranchID)
				if parentBranch := cachedParentBranch.Unwrap(); parentBranch == nil || !parentBranch.Finalized() {
					cachedParentBranch.Release()

					return
				}
				cachedParentBranch.Release()
			}

			// abort if the branch was finalized already
			if !branch.setFinalized(true) {
				return
			}

			// trigger events
			branchManager.Events.BranchFinalized.Trigger(cachedBranch)

			// schedule confirmed checks of children
			branchManager.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
				propagationStack.PushBack(branchManager.Branch(childBranch.childID))
			})
		})
		propagationStack.Remove(stackElement)
	}

	return
}

func (branchManager *BranchManager) propagateRejectedToChildBranches(cachedBranch *CachedBranch) {
	branchStack := list.New()
	branchStack.PushBack(cachedBranch)

	for branchStack.Len() >= 1 {
		currentStackElement := branchStack.Front()
		currentCachedBranch := currentStackElement.Value.(*CachedBranch)
		branchStack.Remove(currentStackElement)

		currentBranch := currentCachedBranch.Unwrap()
		if currentBranch == nil || !currentBranch.setRejected(true) {
			currentCachedBranch.Release()

			continue
		}

		branchManager.Events.BranchRejected.Trigger(cachedBranch)

		branchManager.ChildBranches(currentBranch.ID()).Consume(func(childBranch *ChildBranch) {
			branchStack.PushBack(branchManager.Branch(childBranch.ChildID()))
		})

		currentCachedBranch.Release()
	}
}

func (branchManager *BranchManager) propagateConfirmedToChildBranches(cachedBranch *CachedBranch) (err error) {
	// initialize stack with our entry point for the propagation
	propagationStack := list.New()
	propagationStack.PushBack(cachedBranch)

	// iterate through stack to propagate the changes to child branches
	for propagationStack.Len() >= 1 {
		stackElement := propagationStack.Front()
		stackElement.Value.(*CachedBranch).Consume(func(branch *Branch) {
			// abort if the branch does not fulfill the conditions to be confirmed
			if !branch.Preferred() || !branch.Finalized() {
				return
			}

			// abort if not all parents are confirmed
			for _, parentBranchID := range branch.ParentBranches() {
				cachedParentBranch := branchManager.Branch(parentBranchID)
				if parentBranch := cachedParentBranch.Unwrap(); parentBranch == nil || !parentBranch.Confirmed() {
					cachedParentBranch.Release()

					return
				}
				cachedParentBranch.Release()
			}

			// abort if the branch was confirmed already
			if !branch.setConfirmed(true) {
				return
			}

			// trigger events
			branchManager.Events.BranchConfirmed.Trigger(cachedBranch)

			// schedule confirmed checks of children
			branchManager.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
				propagationStack.PushBack(branchManager.Branch(childBranch.childID))
			})
		})
		propagationStack.Remove(stackElement)
	}

	return
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (branchManager *BranchManager) Prune() (err error) {
	for _, storage := range []*objectstorage.ObjectStorage{
		branchManager.branchStorage,
		branchManager.childBranchStorage,
		branchManager.conflictStorage,
		branchManager.conflictMemberStorage,
	} {
		if err = storage.Prune(); err != nil {
			return
		}
	}

	branchManager.init()

	return
}

func (branchManager *BranchManager) init() {
	cachedBranch, branchAdded := branchManager.Fork(MasterBranchID, []BranchID{}, []ConflictID{})
	if !branchAdded {
		cachedBranch.Release()

		return
	}

	cachedBranch.Consume(func(branch *Branch) {
		branch.setPreferred(true)
		branch.setLiked(true)
		branch.setFinalized(true)
		branch.setConfirmed(true)
	})
}

func (branchManager *BranchManager) setBranchPreferred(cachedBranch *CachedBranch, preferred bool) (modified bool, err error) {
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = fmt.Errorf("failed to unwrap branch")

		return
	}

	if !preferred {
		if modified = branch.setPreferred(false); modified {
			branchManager.Events.BranchUnpreferred.Trigger(cachedBranch)
			branchManager.propagatePreferredChangesToAggregatedChildBranches(branch.ID())
			branchManager.propagateDislikeToFutureCone(cachedBranch.Retain())
		}

		return
	}

	for conflictID := range branch.Conflicts() {
		// update all other branches to be not preferred
		branchManager.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			// skip the branch which just got preferred
			if conflictMember.BranchID() == branch.ID() {
				return
			}

			_, _ = branchManager.setBranchPreferred(branchManager.Branch(conflictMember.BranchID()), false)
		})
	}

	// finally set the branch as preferred
	if modified = branch.setPreferred(true); !modified {
		return
	}

	branchManager.Events.BranchPreferred.Trigger(cachedBranch)
	branchManager.propagatePreferredChangesToAggregatedChildBranches(branch.ID())
	err = branchManager.propagateLike(cachedBranch.Retain())

	return
}

// propagatePreferredChangesToAggregatedChildBranches recursively updates the preferred flag of all descendants of the
// given Branch.
func (branchManager *BranchManager) propagatePreferredChangesToAggregatedChildBranches(parentBranchID BranchID) {
	// initialize stack with children of the passed in parent Branch
	branchStack := list.New()
	branchManager.ChildBranches(parentBranchID).Consume(func(childBranchReference *ChildBranch) {
		branchStack.PushBack(childBranchReference.ChildID())
	})

	// iterate through child branches and propagate changes
	for branchStack.Len() >= 1 {
		// retrieve first element from the stack
		currentEntry := branchStack.Front()
		currentChildBranchID := currentEntry.Value.(BranchID)
		branchStack.Remove(currentEntry)

		// load branch from storage
		cachedAggregatedChildBranch := branchManager.Branch(currentChildBranchID)

		// only process branches that could be loaded and that are aggregated branches
		if aggregatedChildBranch := cachedAggregatedChildBranch.Unwrap(); aggregatedChildBranch != nil && aggregatedChildBranch.IsAggregated() {
			// determine if all parents are liked
			allParentsPreferred := true
			for _, parentID := range aggregatedChildBranch.ParentBranches() {
				if allParentsPreferred {
					allParentsPreferred = allParentsPreferred && branchManager.Branch(parentID).Consume(func(parentBranch *Branch) {
						allParentsPreferred = allParentsPreferred && parentBranch.Preferred()
					})
				}
			}

			// trigger events and check children if the branch was updated
			if aggregatedChildBranch.setPreferred(allParentsPreferred) {
				if allParentsPreferred {
					branchManager.Events.BranchPreferred.Trigger(cachedAggregatedChildBranch)
				} else {
					branchManager.Events.BranchUnpreferred.Trigger(cachedAggregatedChildBranch)
				}

				// schedule checks for children if preferred flag was updated
				branchManager.ChildBranches(currentChildBranchID).Consume(func(childBranchReference *ChildBranch) {
					branchStack.PushBack(childBranchReference.ChildID())
				})
			}
		}

		// release the branch when we are done
		cachedAggregatedChildBranch.Release()
	}
}

func (branchManager *BranchManager) setBranchLiked(cachedBranch *CachedBranch, liked bool) (modified bool, err error) {
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = fmt.Errorf("failed to unwrap branch")

		return
	}

	if !liked {
		if !branch.setPreferred(false) {
			return
		}

		branchManager.Events.BranchUnpreferred.Trigger(cachedBranch)

		if modified = branch.setLiked(false); !modified {
			return
		}

		branchManager.Events.BranchDisliked.Trigger(cachedBranch)

		branchManager.propagateDislikeToFutureCone(cachedBranch.Retain())

		return
	}

	for _, parentBranchID := range branch.ParentBranches() {
		if _, err = branchManager.setBranchLiked(branchManager.Branch(parentBranchID), true); err != nil {
			return
		}
	}

	for conflictID := range branch.Conflicts() {
		branchManager.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			if conflictMember.BranchID() == branch.ID() {
				return
			}

			_, err = branchManager.setBranchPreferred(branchManager.Branch(conflictMember.BranchID()), false)
			if err != nil {
				return
			}
		})
	}

	if !branch.setPreferred(true) {
		return
	}

	branchManager.Events.BranchPreferred.Trigger(cachedBranch)

	if modified = branch.setLiked(true); !modified {
		return
	}

	branchManager.Events.BranchLiked.Trigger(cachedBranch)

	err = branchManager.propagateLike(cachedBranch.Retain())

	return
}

// IsBranchLiked returns true if the Branch is currently marked as liked.
func (branchManager *BranchManager) IsBranchLiked(id BranchID) (liked bool) {
	if id == UndefinedBranchID {
		return
	}

	if id == MasterBranchID {
		return true
	}

	branchManager.Branch(id).Consume(func(branch *Branch) {
		liked = branch.Liked()
	})

	return
}

// IsBranchConfirmed returns true if the Branch is marked as confirmed.
func (branchManager *BranchManager) IsBranchConfirmed(id BranchID) (confirmed bool) {
	if id == UndefinedBranchID {
		return
	}

	if id == MasterBranchID {
		return true
	}

	branchManager.Branch(id).Consume(func(branch *Branch) {
		confirmed = branch.Confirmed()
	})

	return
}

func (branchManager *BranchManager) propagateLike(cachedBranch *CachedBranch) (err error) {
	// unpack CachedBranch and abort of the branch doesn't exist or isn't preferred
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil || !branch.Preferred() {
		return
	}

	// check if parents are liked
	for _, parentBranchID := range branch.ParentBranches() {
		// abort, if the parent branch can not be loaded
		cachedParentBranch := branchManager.Branch(parentBranchID)
		parentBranch := cachedParentBranch.Unwrap()
		if parentBranch == nil {
			cachedParentBranch.Release()

			return fmt.Errorf("failed to load parent branch '%s' of branch '%s'", parentBranchID, branch.ID())
		}

		// abort if the parent branch is not liked
		if !parentBranch.Liked() {
			cachedParentBranch.Release()

			return
		}

		cachedParentBranch.Release()
	}

	// abort if the branch was liked already
	if !branch.setLiked(true) {
		return
	}

	// trigger events
	branchManager.Events.BranchLiked.Trigger(cachedBranch)

	// propagate liked checks to the children
	for _, cachedChildBranch := range branchManager.ChildBranches(branch.ID()) {
		childBranch := cachedChildBranch.Unwrap()
		if childBranch == nil {
			cachedChildBranch.Release()

			continue
		}

		if err = branchManager.propagateLike(branchManager.Branch(childBranch.ChildID())); err != nil {
			cachedChildBranch.Release()

			return
		}

		cachedChildBranch.Release()
	}

	return
}

func (branchManager *BranchManager) propagateDislikeToFutureCone(cachedBranch *CachedBranch) {
	branchStack := list.New()
	branchStack.PushBack(cachedBranch)

	for branchStack.Len() >= 1 {
		currentStackElement := branchStack.Front()
		currentCachedBranch := currentStackElement.Value.(*CachedBranch)
		branchStack.Remove(currentStackElement)

		currentBranch := currentCachedBranch.Unwrap()
		if currentBranch == nil || !currentBranch.setLiked(false) {
			currentCachedBranch.Release()

			continue
		}

		branchManager.Events.BranchDisliked.Trigger(cachedBranch)

		branchManager.ChildBranches(currentBranch.ID()).Consume(func(childBranch *ChildBranch) {
			branchStack.PushBack(branchManager.Branch(childBranch.ChildID()))
		})

		currentCachedBranch.Release()
	}
}

func (branchManager *BranchManager) determineAggregatedBranchDetails(deepestCommonAncestors CachedBranches) (aggregatedBranchID BranchID, aggregatedBranchParents []BranchID, err error) {
	aggregatedBranchParents = make([]BranchID, len(deepestCommonAncestors))

	i := 0
	aggregatedBranchConflictParents := make(CachedBranches)
	for branchID, cachedBranch := range deepestCommonAncestors {
		// release all following entries if we have encountered an error
		if err != nil {
			cachedBranch.Release()

			continue
		}

		// store BranchID as parent
		aggregatedBranchParents[i] = branchID
		i++

		// abort if we could not unwrap the Branch (should never happen)
		branch := cachedBranch.Unwrap()
		if branch == nil {
			cachedBranch.Release()

			err = fmt.Errorf("failed to unwrap brach '%s'", branchID)

			continue
		}

		if !branch.IsAggregated() {
			aggregatedBranchConflictParents[branchID] = cachedBranch

			continue
		}

		err = branchManager.collectClosestConflictAncestors(branch, aggregatedBranchConflictParents)

		cachedBranch.Release()
	}

	if err != nil {
		aggregatedBranchConflictParents.Release()
		aggregatedBranchConflictParents = nil

		return
	}

	aggregatedBranchID = branchManager.generateAggregatedBranchID(aggregatedBranchConflictParents)

	return
}

func (branchManager *BranchManager) generateAggregatedBranchID(aggregatedBranches CachedBranches) BranchID {
	counter := 0
	branchIDs := make([]BranchID, len(aggregatedBranches))
	for branchID, cachedBranch := range aggregatedBranches {
		branchIDs[counter] = branchID

		counter++

		cachedBranch.Release()
	}

	return branchManager.GenerateAggregatedBranchID(branchIDs...)
}

func (branchManager *BranchManager) collectClosestConflictAncestors(branch *Branch, closestConflictAncestors CachedBranches) (err error) {
	// initialize stack
	stack := list.New()
	for _, parentRealityID := range branch.ParentBranches() {
		stack.PushBack(parentRealityID)
	}

	// work through stack
	processedBranches := make(map[BranchID]types.Empty)
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchID := stack.Front().Value.(BranchID)

			// abort if the parent has been processed already
			if _, branchProcessed := processedBranches[parentBranchID]; branchProcessed {
				return nil
			}
			processedBranches[parentBranchID] = types.Void

			// load parent branch from database
			cachedParentBranch := branchManager.Branch(parentBranchID)

			// abort if the parent branch could not be found (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to load branch '%s'", parentBranchID)
			}

			// if the parent Branch is not aggregated, then we have found the closest conflict ancestor
			if !parentBranch.IsAggregated() {
				closestConflictAncestors[parentBranchID] = cachedParentBranch

				return nil
			}

			// queue parents for additional check (recursion)
			for _, parentRealityID := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityID)
			}

			// release the branch (we don't need it anymore)
			cachedParentBranch.Release()

			return nil
		}()

		if err != nil {
			return
		}
	}

	return
}

// findDeepestCommonDescendants takes a number of BranchIds and determines the most specialized Branches (furthest
// away from the MasterBranch) in that list, that contains all of the named BranchIds.
//
// Example: If we hand in "A, B" and B has A as its parent, then the result will contain the Branch B, because B is a
//          child of A.
func (branchManager *BranchManager) findDeepestCommonDescendants(branches ...BranchID) (result CachedBranches, err error) {
	result = make(CachedBranches)

	processedBranches := make(map[BranchID]types.Empty)
	for _, branchID := range branches {
		err = func() error {
			// continue, if we have processed this branch already
			if _, exists := processedBranches[branchID]; exists {
				return nil
			}
			processedBranches[branchID] = types.Void

			// load branch from objectstorage
			cachedBranch := branchManager.Branch(branchID)

			// abort if we could not load the CachedBranch
			branch := cachedBranch.Unwrap()
			if branch == nil {
				cachedBranch.Release()

				return fmt.Errorf("could not load branch '%s'", branchID)
			}

			// check branches position relative to already aggregated branches
			for aggregatedBranchID, cachedAggregatedBranch := range result {
				// abort if we can not load the branch
				aggregatedBranch := cachedAggregatedBranch.Unwrap()
				if aggregatedBranch == nil {
					return fmt.Errorf("could not load branch '%s'", aggregatedBranchID)
				}

				// if the current branch is an ancestor of an already aggregated branch, then we have found the more
				// "specialized" branch already and keep it
				if isAncestor, ancestorErr := branchManager.branchIsAncestorOfBranch(branch, aggregatedBranch); isAncestor || ancestorErr != nil {
					return ancestorErr
				}

				// check if the aggregated Branch is an ancestor of the current Branch and abort if we face an error
				isAncestor, ancestorErr := branchManager.branchIsAncestorOfBranch(aggregatedBranch, branch)
				if ancestorErr != nil {
					return ancestorErr
				}

				// if the aggregated branch is an ancestor of the current branch, then we have found a more specialized
				// Branch and replace the old one with this one.
				if isAncestor {
					// replace aggregated branch if we have found a more specialized on
					delete(result, aggregatedBranchID)
					cachedAggregatedBranch.Release()

					result[branchID] = cachedBranch
				}
			}

			// store the branch as a new aggregate candidate if it was not found to be in any relation with the already
			// aggregated ones.
			result[branchID] = cachedBranch

			return nil
		}()

		// abort if an error occurred while processing the current branch
		if err != nil {
			result.Release()
			result = nil

			return
		}
	}

	return
}

func (branchManager *BranchManager) branchIsAncestorOfBranch(ancestor *Branch, descendant *Branch) (isAncestor bool, err error) {
	if ancestor.ID() == descendant.ID() {
		return true, nil
	}

	ancestorBranches, err := branchManager.getAncestorBranches(descendant)
	if err != nil {
		return
	}

	ancestorBranches.Consume(func(ancestorOfDescendant *Branch) {
		if ancestorOfDescendant.ID() == ancestor.ID() {
			isAncestor = true
		}
	})

	return
}

func (branchManager *BranchManager) getAncestorBranches(branch *Branch) (ancestorBranches CachedBranches, err error) {
	// initialize result
	ancestorBranches = make(CachedBranches)

	// initialize stack
	stack := list.New()
	for _, parentRealityID := range branch.ParentBranches() {
		stack.PushBack(parentRealityID)
	}

	// work through stack
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchID := stack.Front().Value.(BranchID)

			// abort if the parent has been processed already
			if _, branchProcessed := ancestorBranches[parentBranchID]; branchProcessed {
				return nil
			}

			// load parent branch from database
			cachedParentBranch := branchManager.Branch(parentBranchID)

			// abort if the parent branch could not be founds (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to unwrap branch '%s'", parentBranchID)
			}

			// store parent branch in result
			ancestorBranches[parentBranchID] = cachedParentBranch

			// queue parents for additional check (recursion)
			for _, parentRealityID := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityID)
			}

			return nil
		}()

		// abort if an error occurs while trying to process the parents
		if err != nil {
			ancestorBranches.Release()
			ancestorBranches = nil

			return
		}
	}

	return
}
