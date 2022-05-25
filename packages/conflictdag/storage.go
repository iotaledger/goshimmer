package conflictdag

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/serix"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage is a ConflictDAG component that bundles the storage related API.
type Storage[ConflictID comparable, ConflictSetID comparable] struct {
	// branchStorage is an object storage used to persist Conflict objects.
	branchStorage *objectstorage.ObjectStorage[*Conflict[ConflictID, ConflictSetID]]

	// childBranchStorage is an object storage used to persist ChildBranch objects.
	childBranchStorage *objectstorage.ObjectStorage[*ChildBranch[ConflictID]]

	// conflictMemberStorage is an object storage used to persist ConflictMember objects.
	conflictMemberStorage *objectstorage.ObjectStorage[*ConflictMember[ConflictSetID, ConflictID]]

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newStorage returns a new Storage instance configured with the given options.
func newStorage[ConflictID comparable, ConflictSetID comparable](options *options) (new *Storage[ConflictID, ConflictSetID]) {
	new = &Storage[ConflictID, ConflictSetID]{
		branchStorage: objectstorage.NewStructStorage[Conflict[ConflictID, ConflictSetID]](
			objectstorage.NewStoreWithRealm(options.store, database.PrefixConflictDAG, PrefixBranchStorage),
			options.cacheTimeProvider.CacheTime(options.branchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childBranchStorage: objectstorage.NewStructStorage[ChildBranch[ConflictID]](
			objectstorage.NewStoreWithRealm(options.store, database.PrefixConflictDAG, PrefixChildBranchStorage),
			objectstorage.PartitionKey(ChildBranch[ConflictID]{}.PartitionKey()...),
			options.cacheTimeProvider.CacheTime(options.childBranchCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictMemberStorage: objectstorage.NewStructStorage[ConflictMember[ConflictSetID, ConflictID]](
			objectstorage.NewStoreWithRealm(options.store, database.PrefixConflictDAG, PrefixConflictMemberStorage),
			objectstorage.PartitionKey(ConflictMember[ConflictSetID, ConflictID]{}.PartitionKey()...),
			options.cacheTimeProvider.CacheTime(options.conflictMemberCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
	}

	return new
}

// CachedConflict retrieves the CachedObject representing the named Conflict. The optional computeIfAbsentCallback can be
// used to dynamically initialize a non-existing Conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflict(conflictID ConflictID, computeIfAbsentCallback ...func(conflictID ConflictID) *Conflict[ConflictID, ConflictSetID]) (cachedBranch *objectstorage.CachedObject[*Conflict[ConflictID, ConflictSetID]]) {
	conflictIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), conflictID, serix.WithValidation()))

	if len(computeIfAbsentCallback) >= 1 {
		return s.branchStorage.ComputeIfAbsent(conflictIDBytes, func(key []byte) *Conflict[ConflictID, ConflictSetID] {
			return computeIfAbsentCallback[0](conflictID)
		})
	}

	return s.branchStorage.Load(conflictIDBytes)
}

// CachedChildBranch retrieves the CachedObject representing the named ChildBranch. The optional computeIfAbsentCallback
// can be used to dynamically initialize a non-existing ChildBranch.
func (s *Storage[ConflictID, ConflictSetID]) CachedChildBranch(parentBranchID, childBranchID ConflictID, computeIfAbsentCallback ...func(parentBranchID, childBranchID ConflictID) *ChildBranch[ConflictID]) *objectstorage.CachedObject[*ChildBranch[ConflictID]] {
	parentBranchIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), parentBranchID, serix.WithValidation()))
	childBranchIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), childBranchID, serix.WithValidation()))

	if len(computeIfAbsentCallback) >= 1 {
		return s.childBranchStorage.ComputeIfAbsent(byteutils.ConcatBytes(parentBranchIDBytes, childBranchIDBytes), func(key []byte) *ChildBranch[ConflictID] {
			return computeIfAbsentCallback[0](parentBranchID, childBranchID)
		})
	}

	return s.childBranchStorage.Load(byteutils.ConcatBytes(parentBranchIDBytes, childBranchIDBytes))
}

// CachedChildBranches retrieves the CachedObjects containing the ChildBranch references approving the named Conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedChildBranches(branchID ConflictID) (cachedChildBranches objectstorage.CachedObjects[*ChildBranch[ConflictID]]) {
	branchIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), branchID, serix.WithValidation()))

	cachedChildBranches = make(objectstorage.CachedObjects[*ChildBranch[ConflictID]], 0)
	s.childBranchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ChildBranch[ConflictID]]) bool {
		cachedChildBranches = append(cachedChildBranches, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(branchIDBytes))

	return
}

// CachedConflictMember retrieves the CachedObject representing the named ConflictMember. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing ConflictMember.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflictMember(conflictID ConflictSetID, branchID ConflictID, computeIfAbsentCallback ...func(conflictID ConflictSetID, branchID ConflictID) *ConflictMember[ConflictSetID, ConflictID]) *objectstorage.CachedObject[*ConflictMember[ConflictSetID, ConflictID]] {
	conflictIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), conflictID, serix.WithValidation()))
	branchIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), branchID, serix.WithValidation()))

	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictMemberStorage.ComputeIfAbsent(byteutils.ConcatBytes(conflictIDBytes, branchIDBytes), func(key []byte) *ConflictMember[ConflictSetID, ConflictID] {
			return computeIfAbsentCallback[0](conflictID, branchID)
		})
	}

	return s.conflictMemberStorage.Load(byteutils.ConcatBytes(conflictIDBytes, branchIDBytes))
}

// CachedConflictMembers retrieves the CachedObjects containing the ConflictMember references related to the named
// conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflictMembers(conflictID ConflictSetID) (cachedConflictMembers objectstorage.CachedObjects[*ConflictMember[ConflictSetID, ConflictID]]) {
	conflictIDBytes := lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), conflictID, serix.WithValidation()))

	cachedConflictMembers = make(objectstorage.CachedObjects[*ConflictMember[ConflictSetID, ConflictID]], 0)
	s.conflictMemberStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*ConflictMember[ConflictSetID, ConflictID]]) bool {
		cachedConflictMembers = append(cachedConflictMembers, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(conflictIDBytes))

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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixBranchStorage defines the storage prefix for the Conflict object storage.
	PrefixBranchStorage byte = iota
	// PrefixChildBranchStorage defines the storage prefix for the ChildBranch object storage.
	PrefixChildBranchStorage
	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
