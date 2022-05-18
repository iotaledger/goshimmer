package branchdag

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/set"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage is a ConflictDAG component that bundles the storage related API.
type Storage[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// branchStorage is an object storage used to persist Branch objects.
	branchStorage *objectstorage.ObjectStorage[*Branch[ConflictID, ConflictSetID]]

	// childBranchStorage is an object storage used to persist ChildBranch objects.
	childBranchStorage *objectstorage.ObjectStorage[*ChildBranch[ConflictID]]

	// conflictMemberStorage is an object storage used to persist ConflictMember objects.
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember[ConflictID, ConflictSetID]]

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newStorage returns a new Storage instance configured with the given options.
func newStorage[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]](options *options) (new *Storage[ConflictID, ConflictSetID]) {
	var conflictID ConflictID
	var conflictSetID ConflictSetID

	new = &Storage[ConflictID, ConflictSetID]{
		branchStorage: objectstorage.New[*Branch[ConflictID, ConflictSetID]](
			objectstorage.NewStoreWithRealm(options.store, database.PrefixConflictDAG, PrefixBranchStorage),
			options.cacheTimeProvider.CacheTime(options.branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childBranchStorage: objectstorage.New[*ChildBranch[ConflictID]](
			objectstorage.NewStoreWithRealm(options.store, database.PrefixConflictDAG, PrefixChildBranchStorage),
			objectstorage.PartitionKey(len(conflictID.Bytes()), len(conflictID.Bytes())),
			options.cacheTimeProvider.CacheTime(options.childBranchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictMemberStorage: objectstorage.New[*ConflictMember[ConflictID, ConflictSetID]](
			objectstorage.NewStoreWithRealm(options.store, database.PrefixConflictDAG, PrefixConflictMemberStorage),
			objectstorage.PartitionKey(len(conflictSetID.Bytes()), len(conflictID.Bytes())),
			options.cacheTimeProvider.CacheTime(options.conflictMemberCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
	}

	new.init()

	return new
}

// CachedBranch retrieves the CachedObject representing the named Branch. The optional computeIfAbsentCallback can be
// used to dynamically initialize a non-existing Branch.
func (s *Storage[ConflictID, ConflictSetID]) CachedBranch(branchID ConflictID, computeIfAbsentCallback ...func(branchID ConflictID) *Branch[ConflictID, ConflictSetID]) (cachedBranch *objectstorage.CachedObject[*Branch[ConflictID, ConflictSetID]]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *Branch[ConflictID, ConflictSetID] {
			return computeIfAbsentCallback[0](branchID)
		})
	}

	return s.branchStorage.Load(branchID.Bytes())
}

// CachedChildBranch retrieves the CachedObject representing the named ChildBranch. The optional computeIfAbsentCallback
// can be used to dynamically initialize a non-existing ChildBranch.
func (s *Storage[ConflictID, ConflictSetID]) CachedChildBranch(parentBranchID, childBranchID ConflictID, computeIfAbsentCallback ...func(parentBranchID, childBranchID ConflictID) *ChildBranch[ConflictID]) *objectstorage.CachedObject[*ChildBranch[ConflictID]] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.childBranchStorage.ComputeIfAbsent(byteutils.ConcatBytes(parentBranchID.Bytes(), childBranchID.Bytes()), func(key []byte) *ChildBranch[ConflictID] {
			return computeIfAbsentCallback[0](parentBranchID, childBranchID)
		})
	}

	return s.childBranchStorage.Load(byteutils.ConcatBytes(parentBranchID.Bytes(), childBranchID.Bytes()))
}

// CachedChildBranches retrieves the CachedObjects containing the ChildBranch references approving the named Branch.
func (s *Storage[ConflictID, ConflictSetID]) CachedChildBranches(branchID ConflictID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch[ConflictID]]) {
	cachedChildBranches = make(objectstorage.CachedObjects[*ChildBranch[ConflictID]], 0)
	s.childBranchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ChildBranch[ConflictID]]) bool {
		cachedChildBranches = append(cachedChildBranches, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(branchID.Bytes()))

	return
}

// CachedConflictMember retrieves the CachedObject representing the named ConflictMember. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing ConflictMember.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflictMember(conflictID ConflictSetID, branchID ConflictID, computeIfAbsentCallback ...func(conflictID ConflictSetID, branchID ConflictID) *ConflictMember[ConflictID, ConflictSetID]) *objectstorage.CachedObject[*ConflictMember[ConflictID, ConflictSetID]] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictMemberStorage.ComputeIfAbsent(byteutils.ConcatBytes(conflictID.Bytes(), branchID.Bytes()), func(key []byte) *ConflictMember[ConflictID, ConflictSetID] {
			return computeIfAbsentCallback[0](conflictID, branchID)
		})
	}

	return s.conflictMemberStorage.Load(byteutils.ConcatBytes(conflictID.Bytes(), branchID.Bytes()))
}

// CachedConflictMembers retrieves the CachedObjects containing the ConflictMember references related to the named
// conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflictMembers(conflictID ConflictSetID) (cachedConflictMembers objectstorage.CachedObjects[*ConflictMember[ConflictID, ConflictSetID]]) {
	cachedConflictMembers = make(objectstorage.CachedObjects[*ConflictMember[ConflictID, ConflictSetID]], 0)
	s.conflictMemberStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ConflictMember[ConflictID, ConflictSetID]]) bool {
		cachedConflictMembers = append(cachedConflictMembers, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(conflictID.Bytes()))

	return
}

// Prune resets the database and deletes all entities.
func (s *Storage[ConflictID, ConflictSetID]) Prune() (err error) {
	for _, storagePrune := range []func() error{
		s.branchStorage.Prune,
		s.childBranchStorage.Prune,
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

// Shutdown shuts down the KVStores used to persist data.
func (s *Storage[ConflictID, ConflictSetID]) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.branchStorage.Shutdown()
		s.childBranchStorage.Shutdown()
		s.conflictMemberStorage.Shutdown()
	})
}

// init initializes the Storage by creating the entities related to the MasterBranch.
func (s *Storage[ConflictID, ConflictSetID]) init() {
	var rootConflict ConflictID

	cachedMasterBranch, stored := s.branchStorage.StoreIfAbsent(NewBranch[ConflictID, ConflictSetID](rootConflict, set.NewAdvancedSet[ConflictID](), set.NewAdvancedSet[ConflictSetID]()))
	if stored {
		cachedMasterBranch.Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
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
	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
