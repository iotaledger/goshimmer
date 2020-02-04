package database

import (
	"errors"
	"fmt"
)

const (
	// DBVersion defines the version of the database schema this version of GoShimmer supports.
	// everytime there's a breaking change regarding the stored data, this version flag should be adjusted.
	DBVersion = 1
)

var (
	ErrDBVersionIncompatible = errors.New("database version is not compatible. please delete your database folder and restart")
	// the key under which the database is stored
	dbVersionKey = []byte{0}
)

// checks whether the database is compatible with the current schema version.
// also automatically sets the version if the database is new.
func checkDatabaseVersion(dbIsNew bool) {
	dbInstance, err := Get(DBPrefixDatabaseVersion, instance)
	if err != nil {
		panic(err)
	}

	if dbIsNew {
		// store db version for the first time in the new database
		if err = dbInstance.Set(Entry{Key: dbVersionKey, Value: []byte{DBVersion}}); err != nil {
			panic(fmt.Sprintf("unable to persist db version number: %s", err.Error()))
		}
		return
	}

	// db version must be available
	entry, err := dbInstance.Get(dbVersionKey)
	if err != nil {
		if err == ErrKeyNotFound {
			panic(err)
		}
		panic(fmt.Errorf("%w: no database version was persisted", ErrDBVersionIncompatible))
	}
	if entry.Value[0] != DBVersion {
		panic(fmt.Errorf("%w: supported version: %d, version of database: %d", ErrDBVersionIncompatible, DBVersion, entry.Value[0]))
	}
}
