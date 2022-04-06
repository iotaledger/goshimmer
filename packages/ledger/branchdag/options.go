package branchdag

import (
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region WithStore ////////////////////////////////////////////////////////////////////////////////////////////////////

// WithStore is an Option for the BranchDAG that allows to configure which KVStore is supposed to be used to persist
// data (the default option is to use a MapDB).
func WithStore(store kvstore.KVStore) Option {
	return func(options *options) {
		options.store = store
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithCacheTimeProvider ////////////////////////////////////////////////////////////////////////////////////////

// WithCacheTimeProvider is an Option for the BranchDAG that allows to configure which CacheTimeProvider is supposed to
// be used.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *options) {
		options.cacheTimeProvider = cacheTimeProvider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithBranchCacheTime //////////////////////////////////////////////////////////////////////////////////////////

// WithBranchCacheTime is an Option for the BranchDAG that allows to configure how long Branch objects stay cached after
// they have been released.
func WithBranchCacheTime(branchCacheTime time.Duration) Option {
	return func(options *options) {
		options.branchCacheTime = branchCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithChildBranchCacheTime /////////////////////////////////////////////////////////////////////////////////////

// WithChildBranchCacheTime is an Option for the BranchDAG that allows to configure how long ChildBranch objects stay
// cached after they have been released.
func WithChildBranchCacheTime(childBranchCacheTime time.Duration) Option {
	return func(options *options) {
		options.childBranchCacheTime = childBranchCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithConflictMemberCacheTime //////////////////////////////////////////////////////////////////////////////////

// WithConflictMemberCacheTime is an Option for the BranchDAG that allows to configure how long ConflictMember objects
// stay cached after they have been released.
func WithConflictMemberCacheTime(conflictMemberCacheTime time.Duration) Option {
	return func(options *options) {
		options.conflictMemberCacheTime = conflictMemberCacheTime
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Option ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents a configurable parameter for the BranchDAG that modifies its behavior.
type Option func(*options)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region options //////////////////////////////////////////////////////////////////////////////////////////////////////

// options is a container for all configurable parameters of a BranchDAG.
type options struct {
	// store contains the KVStore that is used to persist data.
	store kvstore.KVStore

	// cacheTimeProvider contains the CacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	// branchCacheTime contains the duration that Branch objects stay cached after they have been released.
	branchCacheTime time.Duration

	// childBranchCacheTime contains the duration that ChildBranch objects stay cached after they have been released.
	childBranchCacheTime time.Duration

	// conflictMemberCacheTime contains the duration that ConflictMember objects stay cached after they have been
	// released.
	conflictMemberCacheTime time.Duration
}

// newOptions returns a new options object that corresponds to the handed in options and which is derived from the
// default options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		store:                   mapdb.NewMapDB(),
		cacheTimeProvider:       database.NewCacheTimeProvider(0),
		branchCacheTime:         60 * time.Second,
		childBranchCacheTime:    60 * time.Second,
		conflictMemberCacheTime: 10 * time.Second,
	}).apply(option...)
}

// apply modifies the options object by overriding the handed in options.
func (o *options) apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}
	return o
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
