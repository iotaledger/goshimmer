package database

import (
	"fmt"
	"os"
	"runtime"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/iotaledger/hive.go/kvstore"
	badgerstore "github.com/iotaledger/hive.go/kvstore/badger"
)

const valueLogGCDiscardRatio = 0.1

type badgerDB struct {
	*badger.DB
}

// NewDB returns a new persisting DB object.
func NewDB(dirname string) (DB, error) {
	// assure that the directory exists
	err := createDir(dirname)
	if err != nil {
		return nil, fmt.Errorf("could not create DB directory: %w", err)
	}

	opts := badger.DefaultOptions(dirname)

	opts.Logger = nil
	opts.SyncWrites = false
	opts.TableLoadingMode = options.MemoryMap
	opts.ValueLogLoadingMode = options.MemoryMap
	opts.CompactL0OnClose = false
	opts.KeepL0InMemory = false
	opts.VerifyValueChecksum = false
	opts.ZSTDCompressionLevel = 1
	opts.Compression = options.None
	opts.MaxCacheSize = 50000000
	opts.EventLogging = false

	if runtime.GOOS == "windows" {
		opts = opts.WithTruncate(true)
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open DB: %w", err)
	}

	return &badgerDB{DB: db}, nil
}

func (db *badgerDB) NewStore() kvstore.KVStore {
	return badgerstore.New(db.DB)
}

// Close closes a DB. It's crucial to call it to ensure all the pending updates make their way to disk.
func (db *badgerDB) Close() error {
	return db.DB.Close()
}

func (db *badgerDB) RequiresGC() bool {
	return true
}

func (db *badgerDB) GC() error {
	err := db.RunValueLogGC(valueLogGCDiscardRatio)
	if err != nil {
		return err
	}
	// trigger the go garbage collector to release the used memory
	runtime.GC()
	return nil
}

// Returns whether the given file or directory exists.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, err
}

func createDir(dirname string) error {
	exists, err := exists(dirname)
	if err != nil {
		return err
	}
	if !exists {
		return os.Mkdir(dirname, 0700)
	}
	return nil
}
