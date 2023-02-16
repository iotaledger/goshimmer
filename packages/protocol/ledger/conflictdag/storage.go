package conflictdag

import (
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/objectstorage/generic"
	"sync"

	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/database"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage is a ConflictDAG component that bundles the storage related API.
type Storage[ConflictID comparable, ConflictSetID comparable] struct {
	// conflictStorage is an object storage used to persist Conflict objects.
	conflictStorage *generic.ObjectStorage[*Conflict[ConflictID, ConflictSetID]]

	// childConflictStorage is an object storage used to persist ChildConflict objects.
	childConflictStorage *generic.ObjectStorage[*ChildConflict[ConflictID]]

	// conflictMemberStorage is an object storage used to persist ConflictMember objects.
	conflictMemberStorage *generic.ObjectStorage[*ConflictMember[ConflictSetID, ConflictID]]

	// shutdownOnce is used to ensure that the Shutdown routine is executed only a single time.
	shutdownOnce sync.Once
}

// newStorage returns a new Storage instance configured with the given options.
func newStorage[ConflictID comparable, ConflictSetID comparable](options *optionsConflictDAG) (storage *Storage[ConflictID, ConflictSetID]) {
	storage = &Storage[ConflictID, ConflictSetID]{
		conflictStorage: generic.NewStructStorage[Conflict[ConflictID, ConflictSetID]](
			lo.PanicOnErr(options.store.WithExtendedRealm([]byte{database.PrefixConflictDAG, PrefixConflictStorage})),
			options.cacheTimeProvider.CacheTime(options.conflictCacheTime),
			objectstorage.LeakDetectionEnabled(false),
		),
		childConflictStorage: generic.NewStructStorage[ChildConflict[ConflictID]](
			lo.PanicOnErr(options.store.WithExtendedRealm([]byte{database.PrefixConflictDAG, PrefixChildConflictStorage})),
			objectstorage.PartitionKey(new(ChildConflict[ConflictID]).KeyPartitions()...),
			options.cacheTimeProvider.CacheTime(options.childConflictCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
		conflictMemberStorage: generic.NewStructStorage[ConflictMember[ConflictSetID, ConflictID]](
			lo.PanicOnErr(options.store.WithExtendedRealm([]byte{database.PrefixConflictDAG, PrefixConflictMemberStorage})),
			objectstorage.PartitionKey(new(ConflictMember[ConflictSetID, ConflictID]).KeyPartitions()...),
			options.cacheTimeProvider.CacheTime(options.conflictMemberCacheTime),
			objectstorage.LeakDetectionEnabled(false),
			objectstorage.StoreOnCreation(true),
		),
	}

	return storage
}

// CachedConflict retrieves the CachedObject representing the named Conflict. The optional computeIfAbsentCallback can be
// used to dynamically initialize a non-existing Conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflict(conflictID ConflictID, computeIfAbsentCallback ...func(conflictID ConflictID) *Conflict[ConflictID, ConflictSetID]) (cachedConflict *generic.CachedObject[*Conflict[ConflictID, ConflictSetID]]) {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictStorage.ComputeIfAbsent(bytes(conflictID), func(key []byte) *Conflict[ConflictID, ConflictSetID] {
			return computeIfAbsentCallback[0](conflictID)
		})
	}

	return s.conflictStorage.Load(bytes(conflictID))
}

// CachedChildConflict retrieves the CachedObject representing the named ChildConflict. The optional computeIfAbsentCallback
// can be used to dynamically initialize a non-existing ChildConflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedChildConflict(parentConflictID, childConflictID ConflictID, computeIfAbsentCallback ...func(parentConflictID, childConflictID ConflictID) *ChildConflict[ConflictID]) *generic.CachedObject[*ChildConflict[ConflictID]] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.childConflictStorage.ComputeIfAbsent(byteutils.ConcatBytes(bytes(parentConflictID), bytes(childConflictID)), func(key []byte) *ChildConflict[ConflictID] {
			return computeIfAbsentCallback[0](parentConflictID, childConflictID)
		})
	}

	return s.childConflictStorage.Load(byteutils.ConcatBytes(bytes(parentConflictID), bytes(childConflictID)))
}

// CachedChildConflicts retrieves the CachedObjects containing the ChildConflict references approving the named Conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedChildConflicts(conflictID ConflictID) (cachedChildConflicts generic.CachedObjects[*ChildConflict[ConflictID]]) {
	cachedChildConflicts = make(generic.CachedObjects[*ChildConflict[ConflictID]], 0)
	s.childConflictStorage.ForEach(func(key []byte, cachedObject *generic.CachedObject[*ChildConflict[ConflictID]]) bool {
		cachedChildConflicts = append(cachedChildConflicts, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(bytes(conflictID)))

	return
}

// CachedConflictMember retrieves the CachedObject representing the named ConflictMember. The optional
// computeIfAbsentCallback can be used to dynamically initialize a non-existing ConflictMember.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflictMember(conflictSetID ConflictSetID, conflictID ConflictID, computeIfAbsentCallback ...func(conflictSetID ConflictSetID, conflictID ConflictID) *ConflictMember[ConflictSetID, ConflictID]) *generic.CachedObject[*ConflictMember[ConflictSetID, ConflictID]] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictMemberStorage.ComputeIfAbsent(byteutils.ConcatBytes(bytes(conflictSetID), bytes(conflictID)), func(key []byte) *ConflictMember[ConflictSetID, ConflictID] {
			return computeIfAbsentCallback[0](conflictSetID, conflictID)
		})
	}

	return s.conflictMemberStorage.Load(byteutils.ConcatBytes(bytes(conflictSetID), bytes(conflictID)))
}

// CachedConflictMembers retrieves the CachedObjects containing the ConflictMember references related to the named
// conflict.
func (s *Storage[ConflictID, ConflictSetID]) CachedConflictMembers(conflictID ConflictSetID) (cachedConflictMembers generic.CachedObjects[*ConflictMember[ConflictSetID, ConflictID]]) {
	cachedConflictMembers = make(generic.CachedObjects[*ConflictMember[ConflictSetID, ConflictID]], 0)
	s.conflictMemberStorage.ForEach(func(key []byte, cachedObject *generic.CachedObject[*ConflictMember[ConflictSetID, ConflictID]]) bool {
		cachedConflictMembers = append(cachedConflictMembers, cachedObject)

		return true
	}, objectstorage.WithIteratorPrefix(bytes(conflictID)))

	return
}

// Prune resets the database and deletes all entities.
func (s *Storage[ConflictID, ConflictSetID]) Prune() (err error) {
	for _, storagePrune := range []func() error{
		s.conflictStorage.Prune,
		s.childConflictStorage.Prune,
		s.conflictMemberStorage.Prune,
	} {
		if err = storagePrune(); err != nil {
			err = errors.WithMessagef(cerrors.ErrFatal, "failed to prune the object storage: %s", err.Error())
			return
		}
	}

	return
}

// Shutdown shuts down the KVStores used to persist data.
func (s *Storage[ConflictID, ConflictSetID]) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.conflictStorage.Shutdown()
		s.childConflictStorage.Shutdown()
		s.conflictMemberStorage.Shutdown()
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region db prefixes //////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixConflictStorage defines the storage prefix for the Conflict object storage.
	PrefixConflictStorage byte = iota
	// PrefixChildConflictStorage defines the storage prefix for the ChildConflict object storage.
	PrefixChildConflictStorage
	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
