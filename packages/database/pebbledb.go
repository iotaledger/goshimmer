package database

import (
	"fmt"
	"os"
	"runtime"

	pebbledb "github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/pebble"
)

type pebbleDB struct {
	*pebbledb.DB
}

// NewDB returns a new persisting DB object.
func NewDB(dirname string) (DB, error) {
	// assure that the directory exists
	err := createDir(dirname)
	if err != nil {
		return nil, fmt.Errorf("could not create DB directory: %w", err)
	}

	cache := pebbledb.NewCache(50000000)
	defer cache.Unref()

	opts := &pebbledb.Options{
		Cache:                       cache,
		DisableWAL:                  false,
		L0CompactionThreshold:       2,
		L0StopWritesThreshold:       1000,
		LBaseMaxBytes:               64 << 20, // 64 MB
		Levels:                      make([]pebbledb.LevelOptions, 7),
		MaxConcurrentCompactions:    3,
		MaxOpenFiles:                16384,
		MemTableSize:                64 << 20,
		MemTableStopWritesThreshold: 4,
	}

	for i := 0; i < len(opts.Levels); i++ {
		l := &opts.Levels[i]
		l.BlockSize = 32 << 10       // 32 KB
		l.IndexBlockSize = 256 << 10 // 256 KB
		l.FilterPolicy = bloom.FilterPolicy(10)
		l.FilterType = pebbledb.TableFilter
		if i > 0 {
			l.TargetFileSize = opts.Levels[i-1].TargetFileSize * 2
		}
		l.EnsureDefaults()
	}
	opts.Levels[6].FilterPolicy = nil

	opts.EnsureDefaults()

	db, err := pebble.CreateDB(dirname, opts)
	if err != nil {
		return nil, fmt.Errorf("could not open DB: %w", err)
	}
	return &pebbleDB{DB: db}, nil
}

func (d *pebbleDB) NewStore() kvstore.KVStore {
	return pebble.New(d.DB)
}

// Close closes a DB. It's crucial to call it to ensure all the pending updates make their way to disk.
func (d *pebbleDB) Close() error {
	if err := d.DB.Close(); err != nil {
		return err
	}
	return nil
}

func (d *pebbleDB) RequiresGC() bool {
	return true
}

func (d *pebbleDB) GC() error {
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
