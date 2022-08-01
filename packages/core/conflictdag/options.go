package conflictdag

import (
	"time"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region WithStore ////////////////////////////////////////////////////////////////////////////////////////////////////

// WithStore is an Option for the ConflictDAG that allows to configure which KVStore is supposed to be used to persist
// data (the default option is to use a MapDB).
func WithStore(store kvstore.KVStore) Option {
	return func(options *options) {
		options.store = store
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithCacheTimeProvider ////////////////////////////////////////////////////////////////////////////////////////

// WithCacheTimeProvider is an Option for the ConflictDAG that allows to configure which CacheTimeProvider is supposed to
// be used.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *options) {
		options.cacheTimeProvider = cacheTimeProvider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithConflictCacheTime //////////////////////////////////////////////////////////////////////////////////////////

// WithConflictCacheTime is an Option for the ConflictDAG that allows to configure how long Conflict objects stay cached after
// they have been released.
func WithConflictCacheTime(conflictCacheTime time.Duration) Option {
	return func(options *options) {
		options.conflictCacheTime = conflictCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithChildConflictCacheTime /////////////////////////////////////////////////////////////////////////////////////

// WithChildConflictCacheTime is an Option for the ConflictDAG that allows to configure how long ChildConflict objects stay
// cached after they have been released.
func WithChildConflictCacheTime(childConflictCacheTime time.Duration) Option {
	return func(options *options) {
		options.childConflictCacheTime = childConflictCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithConflictMemberCacheTime //////////////////////////////////////////////////////////////////////////////////

// WithConflictMemberCacheTime is an Option for the ConflictDAG that allows to configure how long ConflictMember objects
// stay cached after they have been released.
func WithConflictMemberCacheTime(conflictMemberCacheTime time.Duration) Option {
	return func(options *options) {
		options.conflictMemberCacheTime = conflictMemberCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithMergeToMaster ////////////////////////////////////////////////////////////////////////////////////////////

// WithMergeToMaster is an Option for the ConflictDAG that allows to configure whether the ConflictDAG should merge
// confirmed Conflicts to the MasterConflict.
func WithMergeToMaster(mergeToMaster bool) Option {
	return func(options *options) {
		options.mergeToMaster = mergeToMaster
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region options //////////////////////////////////////////////////////////////////////////////////////////////////////

// options is a container for all configurable parameters of a ConflictDAG.
type options struct {
	// store contains the KVStore that is used to persist data.
	store kvstore.KVStore

	// cacheTimeProvider contains the CacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	// conflictCacheTime contains the duration that Conflict objects stay cached after they have been released.
	conflictCacheTime time.Duration

	// childConflictCacheTime contains the duration that ChildConflict objects stay cached after they have been released.
	childConflictCacheTime time.Duration

	// conflictMemberCacheTime contains the duration that ConflictMember objects stay cached after they have been
	// released.
	conflictMemberCacheTime time.Duration

	// mergeToMaster contains a boolean flag that indicates whether the ConflictDAG should merge Confirmed Conflicts to the
	// MasterConflict.
	mergeToMaster bool
}

// newOptions returns a new options object that corresponds to the handed in options and which is derived from the
// default options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		store:                   mapdb.NewMapDB(),
		cacheTimeProvider:       database.NewCacheTimeProvider(0),
		conflictCacheTime:       60 * time.Second,
		childConflictCacheTime:  60 * time.Second,
		conflictMemberCacheTime: 10 * time.Second,
		mergeToMaster:           true,
	}).apply(option...)
}

// apply modifies the options object by overriding the handed in options.
func (o *options) apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// Option represents a configurable parameter for the ConflictDAG that modifies its behavior.
type Option func(*options)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
