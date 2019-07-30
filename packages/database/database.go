package database

import (
	"sync"

	"github.com/dgraph-io/badger"
)

var databasesByName = make(map[string]*databaseImpl)
var getLock sync.Mutex

var ErrKeyNotFound = badger.ErrKeyNotFound

type databaseImpl struct {
	db       *badger.DB
	name     string
	prefix   []byte
	openLock sync.Mutex
}

func Get(name string) (Database, error) {
	getLock.Lock()
	defer getLock.Unlock()

	if database, exists := databasesByName[name]; exists {
		return database, nil
	}

	badgerInstance, err := GetBadgerInstance()
	if err != nil {
		return nil, err
	}

	database := &databaseImpl{
		db:     badgerInstance,
		name:   name,
		prefix: []byte(name + "_"),
	}

	databasesByName[name] = database

	return databasesByName[name], nil
}

func (this *databaseImpl) Set(key []byte, value []byte) error {
	if err := this.db.Update(func(txn *badger.Txn) error { return txn.Set(append(this.prefix, key...), value) }); err != nil {
		return err
	}

	return nil
}

func (this *databaseImpl) Contains(key []byte) (bool, error) {
	err := this.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(append(this.prefix, key...))
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
		item, err := txn.Get(append(this.prefix, key...))
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

func (this *databaseImpl) Delete(key []byte) error {
	err := this.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete(append(this.prefix, key...))
		return err
	})
	return err
}

func (this *databaseImpl) ForEach(consumer func([]byte, []byte)) error {
	err := this.db.View(func(txn *badger.Txn) error {
		iteratorOptions := badger.DefaultIteratorOptions
		iteratorOptions.Prefix = this.prefix

		// create an iterator the default options
		it := txn.NewIterator(iteratorOptions)
		defer it.Close()

		// loop through every key-value-pair and call the function
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			consumer(item.Key()[len(this.prefix):], value)
		}
		return nil
	})
	return err
}
