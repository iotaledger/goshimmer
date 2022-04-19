package branchdag

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage is a BranchDAG component that bundles the storage related API.
type Storage struct {
	// branchStorage is an object storage used to persist Branch objects.
	branchStorage *objectstorage.ObjectStorage[*Branch]

	// childBranchStorage is an object storage used to persist ChildBranch objects.
	childBranchStorage *objectstorage.ObjectStorage[*ChildBranch]

	// conflictMemberStorage is an object storage used to persist ConflictMember objects.
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember]

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newStorage returns a new Storage instance configured with the given options.
func newStorage(options *options) (new *Storage) {
	new = &Storage{
		branchStorage: objectstorage.New[*Branch](
			options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixBranchStorage}),
			options.cacheTimeProvider.CacheTime(options.branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childBranchStorage: objectstorage.New[*ChildBranch](
			options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixChildBranchStorage}),
			childBranchKeyPartition,
			options.cacheTimeProvider.CacheTime(options.childBranchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictMemberStorage: objectstorage.New[*ConflictMember](
			options.store.WithRealm([]byte{database.PrefixBranchDAG, PrefixConflictMemberStorage}),
			conflictMemberKeyPartition,
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
func (s *Storage) CachedBranch(branchID BranchID, computeIfAbsentCallback ...func(branchID BranchID) *Branch) (cachedBranch *objectstorage.CachedObject[*Branch]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *Branch {
			return computeIfAbsentCallback[0](branchID)
		})
	}

	return s.branchStorage.Load(branchID.Bytes())
}

// CachedChildBranch retrieves the CachedObject representing the named ChildBranch. The optional computeIfAbsentCallback
// can be used to dynamically initialize a non-existing ChildBranch.
func (s *Storage) CachedChildBranch(parentBranchID, childBranchID BranchID, computeIfAbsentCallback ...func(parentBranchID, childBranchID BranchID) *ChildBranch) *objectstorage.CachedObject[*ChildBranch] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.childBranchStorage.ComputeIfAbsent(byteutils.ConcatBytes(parentBranchID.Bytes(), childBranchID.Bytes()), func(key []byte) *ChildBranch {
			return computeIfAbsentCallback[0](parentBranchID, childBranchID)
		})
	}

	return s.childBranchStorage.Load(byteutils.ConcatBytes(parentBranchID.Bytes(), childBranchID.Bytes()))
}

// CachedChildBranches retrieves the CachedObjects containing the ChildBranch references approving the named Branch.
func (s *Storage) CachedChildBranches(branchID BranchID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch]) {
	cachedChildBranches = make(objectstorage.CachedObjects[*ChildBranch], 0)
	s.childBranchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ChildBranch]) bool {
		cachedChildBranches = append(cachedChildBranches, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(branchID.Bytes()))

	return
}

// CachedConflictMember retrieves the CachedObject representing the named ConflictMember. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing ConflictMember.
func (s *Storage) CachedConflictMember(conflictID ConflictID, branchID BranchID, computeIfAbsentCallback ...func(conflictID ConflictID, branchID BranchID) *ConflictMember) *objectstorage.CachedObject[*ConflictMember] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictMemberStorage.ComputeIfAbsent(byteutils.ConcatBytes(conflictID.Bytes(), branchID.Bytes()), func(key []byte) *ConflictMember {
			return computeIfAbsentCallback[0](conflictID, branchID)
		})
	}

	return s.conflictMemberStorage.Load(byteutils.ConcatBytes(conflictID.Bytes(), branchID.Bytes()))
}

// CachedConflictMembers retrieves the CachedObjects containing the ConflictMember references related to the named
// conflict.
func (s *Storage) CachedConflictMembers(conflictID ConflictID) (cachedConflictMembers objectstorage.CachedObjects[*ConflictMember]) {
	cachedConflictMembers = make(objectstorage.CachedObjects[*ConflictMember], 0)
	s.conflictMemberStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ConflictMember]) bool {
		cachedConflictMembers = append(cachedConflictMembers, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(conflictID.Bytes()))

	return
}

// Prune resets the database and deletes all entities.
func (s *Storage) Prune() (err error) {
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

// Shutdown shuts down the KVStores that are used to persist data.
func (s *Storage) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.branchStorage.Shutdown()
		s.childBranchStorage.Shutdown()
		s.conflictMemberStorage.Shutdown()
	})
}

// init initializes the Storage by creating the entities related to the MasterBranch.
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
	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
