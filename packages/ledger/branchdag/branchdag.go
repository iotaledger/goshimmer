package branchdag

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region BranchDAG ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchDAG represents the DAG of Branches which contains the business logic to manage the creation and maintenance of
// the Branches which represents containers for the different perceptions of the ledger state that exist in the tangle.
type BranchDAG struct {
	Events  *Events
	Storage *Storage

	options             *Options
	shutdownOnce        sync.Once
	inclusionStateMutex sync.RWMutex
}

// NewBranchDAG returns a new BranchDAG instance that stores its state in the given KVStore.
func NewBranchDAG(options ...Option) (newBranchDAG *BranchDAG) {
	newBranchDAG = &BranchDAG{
		Events:  NewEvents(),
		options: NewOptions(options...),
	}
	newBranchDAG.Storage = NewStorage(newBranchDAG)

	return
}

// CreateBranch retrieves the Branch that corresponds to the given details. It automatically creates and
// updates the Branch according to the new details if necessary.
func (b *BranchDAG) CreateBranch(branchID BranchID, parentBranchIDs BranchIDs, conflictIDs ConflictIDs) (created bool) {
	b.inclusionStateMutex.RLock()

	// create or load the branch
	b.Storage.Branch(branchID, func() *Branch {
		branch := NewBranch(branchID, parentBranchIDs, conflictIDs)

		created = true

		return branch
	}).Consume(func(branch *Branch) {
		// If the branch existed already we simply update its conflict members.
		//
		// An existing Branch can only become a new member of a conflict set if that conflict set was newly created in which
		// case none of the members of that set can either be Confirmed or Rejected. This means that our InclusionState does
		// not change, and we don't need to update and propagate it.
		if !created {
			_ = conflictIDs.ForEach(func(conflictID ConflictID) (err error) {
				if branch.AddConflict(conflictID) {
					b.registerConflictMember(conflictID, branchID)
				}

				return nil
			})
			return
		}

		// store child references
		_ = parentBranchIDs.ForEach(func(parentBranchID BranchID) (err error) {
			if cachedChildBranch, stored := b.Storage.childBranchStorage.StoreIfAbsent(NewChildBranch(parentBranchID, branchID)); stored {
				cachedChildBranch.Release()
			}
			return nil
		})

		// store ConflictMember references
		_ = conflictIDs.ForEach(func(conflictID ConflictID) (err error) {
			b.registerConflictMember(conflictID, branchID)
			return nil
		})

		if b.anyParentRejected(branch) || b.anyConflictMemberConfirmed(branch) {
			branch.setInclusionState(Rejected)
		}
	})

	b.inclusionStateMutex.RUnlock()

	if created {
		b.Events.BranchCreated.Trigger(branchID)
	}

	return created
}

// UpdateParentsAfterFork changes the parents of a Branch (also updating the references of the ChildBranches).
func (b *BranchDAG) UpdateParentsAfterFork(branchID, newParentBranchID BranchID, previousParents BranchIDs) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	b.Storage.Branch(branchID).Consume(func(branch *Branch) {
		parentBranchIDs := branch.Parents()
		if !parentBranchIDs.Add(newParentBranchID) {
			return
		}

		parentBranchIDs.DeleteAll(previousParents)

		if cachedChildBranch, stored := b.Storage.childBranchStorage.StoreIfAbsent(NewChildBranch(newParentBranchID, branchID)); stored {
			cachedChildBranch.Release()
		}

		if branch.SetParents(parentBranchIDs) {
			b.Events.BranchParentsUpdated.Trigger(&BranchParentUpdate{branchID, parentBranchIDs})
		}
	})
}

// RemoveConfirmedBranches returns the BranchIDs of the pending and rejected Branches that are
// addressed by the given BranchIDs.
func (b *BranchDAG) RemoveConfirmedBranches(branchIDs BranchIDs) (pendingBranchIDs BranchIDs) {
	pendingBranchIDs = NewBranchIDs()

	branchWalker := walker.New[BranchID]().PushAll(branchIDs.Slice()...)
	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next()

		b.Storage.Branch(currentBranchID).Consume(func(branch *Branch) {
			if branch.InclusionState() == Confirmed {
				return
			}
			pendingBranchIDs.Add(branch.ID())
		})
	}

	if pendingBranchIDs.Size() == 0 {
		pendingBranchIDs = NewBranchIDs(MasterBranchID)
	}

	return pendingBranchIDs
}

// SetBranchConfirmed sets the InclusionState of the given Branch to be Confirmed.
func (b *BranchDAG) SetBranchConfirmed(branchID BranchID) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	confirmationWalker := walker.New[BranchID]().Push(branchID)
	rejectedWalker := walker.New[BranchID]()

	for confirmationWalker.HasNext() {
		currentBranchID := confirmationWalker.Next()

		b.Storage.Branch(currentBranchID).Consume(func(branch *Branch) {
			if modified = branch.setInclusionState(Confirmed); !modified {
				return
			}

			_ = branch.Parents().ForEach(func(branchID BranchID) (err error) {
				confirmationWalker.Push(branchID)
				return nil
			})

			_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
				b.Storage.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
					if conflictMember.BranchID() != currentBranchID {
						rejectedWalker.Push(conflictMember.BranchID())
					}
				})

				return nil
			})
		})
	}

	for rejectedWalker.HasNext() {
		b.Storage.Branch(rejectedWalker.Next()).Consume(func(branch *Branch) {
			if modified = branch.setInclusionState(Rejected); !modified {
				return
			}

			b.Storage.ChildBranches(branch.ID()).Consume(func(childBranch *ChildBranch) {
				rejectedWalker.Push(childBranch.ChildBranchID())
			})
		})
	}

	return modified
}

// InclusionState returns the InclusionState of the given BranchIDs.
func (b *BranchDAG) InclusionState(branchIDs BranchIDs) (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	inclusionState = Confirmed
	_ = branchIDs.ForEach(func(branchID BranchID) (err error) {
		switch b.inclusionState(branchID) {
		case Rejected:
			inclusionState = Rejected
			return errors.New("abort")
		case Pending:
			inclusionState = Pending
		}

		return nil
	})

	return inclusionState
}

func (b *BranchDAG) Shutdown() {
	b.Storage.Shutdown()
}

// inclusionState returns the InclusionState of the given BranchID.
func (b *BranchDAG) inclusionState(branchID BranchID) (inclusionState InclusionState) {
	if !b.Storage.Branch(branchID).Consume(func(branch *Branch) {
		inclusionState = branch.InclusionState()
	}) {
		panic(fmt.Sprintf("failed to load %s", branchID))
	}

	return inclusionState
}

func (b *BranchDAG) anyParentRejected(conflictBranch *Branch) (parentRejected bool) {
	_ = conflictBranch.Parents().ForEach(func(parentBranchID BranchID) (err error) {
		b.Storage.Branch(parentBranchID).Consume(func(parentBranch *Branch) {
			if parentRejected = parentBranch.InclusionState() == Rejected; parentRejected {
				return
			}
		})

		if parentRejected {
			return errors.New("abort")
		}

		return nil
	})

	return
}

// anyConflictMemberConfirmed makes a Branch rejected if any of its conflicting Branches is
// Confirmed.
func (b *BranchDAG) anyConflictMemberConfirmed(branch *Branch) (conflictMemberConfirmed bool) {
	_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
		b.Storage.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
			if conflictMemberConfirmed || conflictMember.BranchID() == branch.ID() {
				return
			}

			b.Storage.Branch(conflictMember.BranchID()).Consume(func(conflictingBranch *Branch) {
				conflictMemberConfirmed = conflictingBranch.InclusionState() == Confirmed
			})
		})

		if conflictMemberConfirmed {
			return errors.New("abort")
		}

		return nil
	})

	return
}

// registerConflictMember is an internal utility function that creates the ConflictMember references of a Branch
// belonging to a given Conflict. It automatically creates the Conflict if it doesn't exist, yet.
func (b *BranchDAG) registerConflictMember(conflictID ConflictID, branchID BranchID) {
	b.Storage.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) *Conflict {
		newConflict := NewConflict(conflictID)
		newConflict.Persist()
		newConflict.SetModified()

		return newConflict
	}).Consume(func(conflict *Conflict) {
		if cachedConflictMember, stored := b.Storage.conflictMemberStorage.StoreIfAbsent(NewConflictMember(conflictID, branchID)); stored {
			conflict.IncreaseMemberCount()

			cachedConflictMember.Release()
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Options is a container for all configurable parameters of the BranchDAG.
type Options struct {
	Store             kvstore.KVStore
	CacheTimeProvider *database.CacheTimeProvider
}

func NewOptions(options ...Option) (new *Options) {
	return (&Options{
		Store:             mapdb.NewMapDB(),
		CacheTimeProvider: database.NewCacheTimeProvider(0),
	}).Apply(options...)
}

func (o *Options) Apply(options ...Option) (self *Options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// Option represents the return type of optional parameters that can be handed into the constructor of the Ledger
// to configure its behavior.
type Option func(*Options)

// WithStore is an Option for the Ledger that allows to specify which storage layer is supposed to be used to persist
// data.
func WithStore(store kvstore.KVStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}

// WithCacheTimeProvider is an Option for the Tangle that allows to override hard coded cache time.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *Options) {
		options.CacheTimeProvider = cacheTimeProvider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
