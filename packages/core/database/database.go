package database

import (
	"github.com/iotaledger/hive.go/kvstore"
)

// DB represents a database abstraction.
type DB interface {
	// NewStore creates a new KVStore backed by the database.
	NewStore() kvstore.KVStore
	// Close closes a DB.
	Close() error

	// RequiresGC returns whether the database requires a call of GC() to clean deleted items.
	RequiresGC() bool
	// GC runs the garbage collection to clean deleted database items.
	GC() error
}
