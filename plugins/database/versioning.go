package database

import (
	"errors"
	"fmt"

	"github.com/iotaledger/hive.go/kvstore"
)

const (
	// DBVersion defines the version of the database schema this version of GoShimmer supports.
	// Every time there's a breaking change regarding the stored data, this version flag should be adjusted.
	DBVersion = 2
)

var (
	// ErrDBVersionIncompatible is returned when the database has an unexpected version.
	ErrDBVersionIncompatible = errors.New("database version is not compatible. please delete your database folder and restart")
	// the key under which the database is stored
	dbVersionKey = []byte{0}
)

// checks whether the database is compatible with the current schema version.
// also automatically sets the version if the database is new.
func checkDatabaseVersion(store kvstore.KVStore) error {
	entry, err := store.Get(dbVersionKey)
	if err == kvstore.ErrKeyNotFound {
		// set the version in an empty DB
		return store.Set(dbVersionKey, []byte{DBVersion})
	}
	if err != nil {
		return err
	}
	if len(entry) == 0 {
		return fmt.Errorf("%w: no database version was persisted", ErrDBVersionIncompatible)
	}
	if entry[0] != DBVersion {
		return fmt.Errorf("%w: supported version: %d, version of database: %d", ErrDBVersionIncompatible, DBVersion, entry[0])
	}
	return nil
}
