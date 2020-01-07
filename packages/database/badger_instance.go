package database

import (
	"os"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/pkg/errors"
)

var instance *badger.DB
var once sync.Once

// Returns whether the given file or directory exists.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func checkDir(dir string) error {
	exists, err := exists(dir)
	if err != nil {
		return err
	}

	if !exists {
		return os.Mkdir(dir, 0700)
	}
	return nil
}

func createDB() (*badger.DB, error) {
	directory := parameter.NodeConfig.GetString(CFG_DIRECTORY)
	if err := checkDir(directory); err != nil {
		return nil, errors.Wrap(err, "Could not check directory")
	}

	opts := badger.DefaultOptions(directory)
	opts.Logger = &logger{}
	opts.Truncate = true
	opts.TableLoadingMode = options.MemoryMap

	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.Wrap(err, "Could not open new DB")
	}

	return db, nil
}

func GetBadgerInstance() *badger.DB {
	once.Do(func() {
		db, err := createDB()
		if err != nil {
			// errors should cause a panic to avoid singleton deadlocks
			panic(err)
		}
		instance = db
	})
	return instance
}
