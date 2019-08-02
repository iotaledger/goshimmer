package database

import (
	"os"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
)

var instance *badger.DB

var openLock sync.Mutex

func GetBadgerInstance() (result *badger.DB, err error) {
	openLock.Lock()

	if instance == nil {
		directory := *DIRECTORY.Value

		if _, osErr := os.Stat(directory); os.IsNotExist(osErr) {
			if osErr := os.Mkdir(directory, 0700); osErr != nil {
				err = osErr

				return
			}
		} else if osErr != nil {
			err = osErr

			return
		}

		opts := badger.DefaultOptions(directory)
		opts.Logger = &logger{}
		opts.Truncate = true
		opts.TableLoadingMode = options.MemoryMap

		db, badgerErr := badger.Open(opts)
		if badgerErr != nil {
			err = badgerErr

			return
		}

		instance = db
	}

	openLock.Unlock()

	result = instance

	return
}
