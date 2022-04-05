package branchdag

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	branchStorage         *objectstorage.ObjectStorage[*Branch]
	childBranchStorage    *objectstorage.ObjectStorage[*ChildBranch]
	conflictStorage       *objectstorage.ObjectStorage[*Conflict]
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember]
	branchDAG             *BranchDAG
	shutdownOnce          sync.Once
}

func newStorage(branchDAG *BranchDAG) (new *Storage) {
	new = &Storage{
		branchStorage: objectstorage.New[*Branch](
			branchDAG.options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixBranchStorage}),
			branchDAG.options.cacheTimeProvider.CacheTime(branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childBranchStorage: objectstorage.New[*ChildBranch](
			branchDAG.options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixChildBranchStorage}),
			ChildBranchKeyPartition,
			branchDAG.options.cacheTimeProvider.CacheTime(branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictStorage: objectstorage.New[*Conflict](
			branchDAG.options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixConflictStorage}),
			branchDAG.options.cacheTimeProvider.CacheTime(consumerCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		conflictMemberStorage: objectstorage.New[*ConflictMember](
			branchDAG.options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixConflictMemberStorage}),
			ConflictMemberKeyPartition,
			branchDAG.options.cacheTimeProvider.CacheTime(conflictCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		branchDAG: branchDAG,
	}

	new.init()

	return new
}

// CachedBranch retrieves the Branch with the given BranchID from the object storage.
func (s *Storage) CachedBranch(branchID BranchID, computeIfAbsentCallback ...func() *Branch) (cachedBranch *objectstorage.CachedObject[*Branch]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *Branch {
			return computeIfAbsentCallback[0]()
		})
	}

	return s.branchStorage.Load(branchID.Bytes())
}

// CachedChildBranches loads the references to the CachedChildBranches of the given Branch from the object storage.
func (s *Storage) CachedChildBranches(branchID BranchID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch]) {
	cachedChildBranches = make(objectstorage.CachedObjects[*ChildBranch], 0)
	s.childBranchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ChildBranch]) bool {
		cachedChildBranches = append(cachedChildBranches, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(branchID.Bytes()))

	return
}

// CachedConflict loads a Conflict from the object storage.
func (s *Storage) CachedConflict(conflictID ConflictID, computeIfAbsentCallback ...func(conflictID ConflictID) *Conflict) *objectstorage.CachedObject[*Conflict] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) *Conflict {
			return computeIfAbsentCallback[0](conflictID)
		})
	}

	return s.conflictStorage.Load(conflictID.Bytes())
}

// CachedConflictMember loads a cached ConflictMember from the object storage.
func (s *Storage) CachedConflictMember(conflictID ConflictID, branchID BranchID, computeIfAbsentCallback ...func(conflictID ConflictID, branchID BranchID) *ConflictMember) *objectstorage.CachedObject[*ConflictMember] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictMemberStorage.ComputeIfAbsent(byteutils.ConcatBytes(conflictID.Bytes(), branchID.Bytes()), func(key []byte) *ConflictMember {
			return computeIfAbsentCallback[0](conflictID, branchID)
		})
	}

	return s.conflictMemberStorage.Load(byteutils.ConcatBytes(conflictID.Bytes(), branchID.Bytes()))
}

// CachedConflictMembers loads the referenced ConflictMembers of a Conflict from the object storage.
func (s *Storage) CachedConflictMembers(conflictID ConflictID) (cachedConflictMembers objectstorage.CachedObjects[*ConflictMember]) {
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
