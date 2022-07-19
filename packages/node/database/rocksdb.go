package database

import (
	"runtime"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
)

// const valueLogGCDiscardRatio = 0.1

type rocksDB struct {
	*rocksdb.RocksDB
}

// NewDB returns a new persisting DB object.
func NewDB(dirname string) (DB, error) {
	db, err := rocksdb.CreateDB(dirname)
	return &rocksDB{RocksDB: db}, err
}

func (db *rocksDB) NewStore() kvstore.KVStore {
	return rocksdb.New(db.RocksDB)
}

// Close closes a DB. It's crucial to call it to ensure all the pending updates make their way to disk.
func (db *rocksDB) Close() error {
	return db.RocksDB.Close()
}

func (db *rocksDB) RequiresGC() bool {
	return true
}

func (db *rocksDB) GC() error {
	// trigger the go garbage collector to release the used memory
	runtime.GC()
	return nil
}
