package database

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

type memDB struct {
	*mapdb.MapDB
}

// NewMemDB returns a new in-memory (not persisted) DB object.
func NewMemDB() (DB, error) {
	return &memDB{MapDB: mapdb.NewMapDB()}, nil
}

func (db *memDB) NewStore() kvstore.KVStore {
	return db.MapDB
}

func (db *memDB) Close() error {
	db.MapDB = nil
	return nil
}

func (db *memDB) RequiresGC() bool {
	return false
}

func (db *memDB) GC() error {
	return nil
}
