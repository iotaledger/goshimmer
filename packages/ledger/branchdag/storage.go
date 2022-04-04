package branchdag

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	*BranchDAG

	branchStorage         *objectstorage.ObjectStorage[*Branch]
	childBranchStorage    *objectstorage.ObjectStorage[*ChildBranch]
	conflictStorage       *objectstorage.ObjectStorage[*Conflict]
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember]
}

func NewStorage(branchDAG *BranchDAG) (new *Storage) {
	new = &Storage{
		BranchDAG: branchDAG,

		branchStorage: objectstorage.New[*Branch](
			branchDAG.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixBranchStorage}),
			branchDAG.options.CacheTimeProvider.CacheTime(branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childBranchStorage: objectstorage.New[*ChildBranch](
			branchDAG.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixChildBranchStorage}),
			ChildBranchKeyPartition,
			branchDAG.options.CacheTimeProvider.CacheTime(branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictStorage: objectstorage.New[*Conflict](
			branchDAG.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixConflictStorage}),
			branchDAG.options.CacheTimeProvider.CacheTime(consumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		conflictMemberStorage: objectstorage.New[*ConflictMember](
			branchDAG.options.Store.WithRealm([]byte{database.PrefixLedger, PrefixConflictMemberStorage}),
			ConflictMemberKeyPartition,
			branchDAG.options.CacheTimeProvider.CacheTime(conflictCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
	}

	new.init()

	return new
}

// Branch retrieves the Branch with the given BranchID from the object storage.
func (s *Storage) Branch(branchID BranchID, computeIfAbsentCallback ...func() *Branch) (cachedBranch *objectstorage.CachedObject[*Branch]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *Branch {
			return computeIfAbsentCallback[0]()
		})
	}

	return s.branchStorage.Load(branchID.Bytes())
}

// ChildBranches loads the references to the ChildBranches of the given Branch from the object storage.
func (s *Storage) ChildBranches(branchID BranchID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch]) {
	cachedChildBranches = make(objectstorage.CachedObjects[*ChildBranch], 0)
	s.childBranchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ChildBranch]) bool {
		cachedChildBranches = append(cachedChildBranches, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(branchID.Bytes()))

	return
}

// ForEachBranch iterates over all the branches and executes consumer.
func (s *Storage) ForEachBranch(consumer func(branch *Branch)) {
	s.branchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Branch]) bool {
		cachedObject.Consume(func(branch *Branch) {
			consumer(branch)
		})

		return true
	})
}

// Conflict loads a Conflict from the object storage.
func (s *Storage) Conflict(conflictID ConflictID) *objectstorage.CachedObject[*Conflict] {
	return s.conflictStorage.Load(conflictID.Bytes())
}

// ConflictMembers loads the referenced ConflictMembers of a Conflict from the object storage.
func (s *Storage) ConflictMembers(conflictID ConflictID) (cachedConflictMembers objectstorage.CachedObjects[*ConflictMember]) {
	cachedConflictMembers = make(objectstorage.CachedObjects[*ConflictMember], 0)
	s.conflictMemberStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ConflictMember]) bool {
		cachedConflictMembers = append(cachedConflictMembers, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(conflictID.Bytes()))

	return
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (s *Storage) Prune() (err error) {
	for _, storagePrune := range []func() error{
		s.branchStorage.Prune,
		s.childBranchStorage.Prune,
		s.conflictStorage.Prune,
		s.conflictMemberStorage.Prune,
	} {
		if err = storagePrune(); err != nil {
			err = errors.Errorf("failed to prune the object storage (%v): %w", err, cerrors.ErrFatal)
			return
		}
	}

	s.init()

	return
}

// Shutdown shuts down the BranchDAG and persists its state.
func (s *Storage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.branchStorage.Shutdown()
		s.childBranchStorage.Shutdown()
		s.conflictStorage.Shutdown()
		s.conflictMemberStorage.Shutdown()
	})
}

func (s *Storage) init() {
	cachedMasterBranch, stored := s.branchStorage.StoreIfAbsent(NewBranch(MasterBranchID, NewBranchIDs(), NewConflictIDs()))
	if stored {
		cachedMasterBranch.Consume(func(branch *Branch) {
			branch.setInclusionState(Confirmed)
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixBranchStorage defines the storage prefix for the Branch object storage.
	PrefixBranchStorage byte = iota

	// PrefixChildBranchStorage defines the storage prefix for the ChildBranch object storage.
	PrefixChildBranchStorage

	// PrefixConflictStorage defines the storage prefix for the Conflict object storage.
	PrefixConflictStorage

	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and configurations /////////////////////////////////////////////////////////////////////////////////

const (
	branchCacheTime   = 60 * time.Second
	conflictCacheTime = 60 * time.Second
	consumerCacheTime = 10 * time.Second
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
