package database

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
)

var databasesByName = make(map[string]*databaseImpl)
var getLock sync.Mutex

var ErrKeyNotFound = badger.ErrKeyNotFound

type databaseImpl struct {
	db       *badger.DB
	name     string
	openLock sync.Mutex
}

func Get(name string) (Database, error) {
	getLock.Lock()
	defer getLock.Unlock()

	if database, exists := databasesByName[name]; exists {
		return database, nil
	}

	database := &databaseImpl{
		db:   nil,
		name: name,
	}
	if err := database.Open(); err != nil {
		return nil, err
	}

	databasesByName[name] = database

	return databasesByName[name], nil
}

func (this *databaseImpl) Open() error {
	this.openLock.Lock()
	defer this.openLock.Unlock()

	if this.db == nil {
		directory := *DIRECTORY.Value

		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err := os.Mkdir(directory, 0700); err != nil {
				return err
			}
		}

		opts := badger.DefaultOptions
		opts.Dir = directory + string(filepath.Separator) + this.name
		opts.ValueDir = opts.Dir
		opts.Logger = &logger{}
		opts.Truncate = true
		opts.TableLoadingMode = options.MemoryMap

		db, err := badger.Open(opts)
		if err != nil {
			return err
		}
		this.db = db
	}

	return nil
}

func (this *databaseImpl) Set(key []byte, value []byte) error {
	if err := this.db.Update(func(txn *badger.Txn) error { return txn.Set(key, value) }); err != nil {
		return err
	}

	return nil
}

func (this *databaseImpl) Contains(key []byte) (bool, error) {
	err := this.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err != nil {
			return err
		}

		return nil
	})

	if err == ErrKeyNotFound {
		return false, nil
	} else {
		return err == nil, err
	}
}

func (this *databaseImpl) Get(key []byte) ([]byte, error) {
	var result []byte = nil

	err := this.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			result = append([]byte{}, val...)

			return nil
		})
	})

	return result, err
}

func (this *databaseImpl) Close() error {
	this.openLock.Lock()
	defer this.openLock.Unlock()

	if this.db != nil {
		err := this.db.Close()

		this.db = nil

		if err != nil {
			return err
		}
	}

	return nil
}
