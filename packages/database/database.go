package database

import (
	"sync"

	"github.com/dgraph-io/badger"
)

var dbMap = make(map[string]*prefixDb)
var mu sync.Mutex

type prefixDb struct {
	db       *badger.DB
	name     string
	prefix   []byte
	openLock sync.Mutex
}

func Get(name string) (Database, error) {
	// avoid locking if it's a clean hit
	if db, exists := dbMap[name]; exists {
		return db, nil
	}

	mu.Lock()
	defer mu.Unlock()

	// needs to be re-checked after locking
	if db, exists := dbMap[name]; exists {
		return db, nil
	}

	badger := GetBadgerInstance()
	db := &prefixDb{
		db:     badger,
		name:   name,
		prefix: []byte(name + "_"),
	}

	dbMap[name] = db

	return db, nil
}

func (this *prefixDb) Set(key []byte, value []byte) error {
	if err := this.db.Update(func(txn *badger.Txn) error { return txn.Set(append(this.prefix, key...), value) }); err != nil {
		return err
	}

	return nil
}

func (this *prefixDb) Contains(key []byte) (bool, error) {
	err := this.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(append(this.prefix, key...))
		if err != nil {
			return err
		}

		return nil
	})

	if err == badger.ErrKeyNotFound {
		return false, nil
	} else {
		return err == nil, err
	}
}

func (this *prefixDb) Get(key []byte) ([]byte, error) {
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

func (this *prefixDb) Delete(key []byte) error {
	err := this.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete(append(this.prefix, key...))
		return err
	})
	return err
}

func (this *prefixDb) ForEach(consumer func([]byte, []byte)) error {
	err := this.db.View(func(txn *badger.Txn) error {
		iteratorOptions := badger.DefaultIteratorOptions
		iteratorOptions.Prefix = this.prefix // filter by prefix

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
