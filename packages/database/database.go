package database

import (
	"sync"
	"time"

	"github.com/dgraph-io/badger"
)

var (
	ErrKeyNotFound = badger.ErrKeyNotFound

	dbMap = make(map[string]*prefixDb)
	mu    sync.Mutex
)

type prefixDb struct {
	db     *badger.DB
	name   string
	prefix []byte
}

func getPrefix(name string) []byte {
	return []byte(name + "_")
}

func Get(name string) (Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if db, exists := dbMap[name]; exists {
		return db, nil
	}

	badger := GetBadgerInstance()
	db := &prefixDb{
		db:     badger,
		name:   name,
		prefix: getPrefix(name),
	}

	dbMap[name] = db

	return db, nil
}

func (this *prefixDb) setEntry(e *badger.Entry) error {
	err := this.db.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(e)
	})
	return err
}

func (this *prefixDb) Set(key []byte, value []byte) error {
	e := badger.NewEntry(append(this.prefix, key...), value)
	return this.setEntry(e)
}

func (this *prefixDb) SetWithTTL(key []byte, value []byte, ttl time.Duration) error {
	e := badger.NewEntry(append(this.prefix, key...), value).WithTTL(ttl)
	return this.setEntry(e)
}

func (this *prefixDb) Contains(key []byte) (bool, error) {
	err := this.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(append(this.prefix, key...))
		return err
	})

	if err == ErrKeyNotFound {
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
		return txn.Delete(append(this.prefix, key...))
	})
	return err
}

func (this *prefixDb) forEach(prefix []byte, consumer func([]byte, []byte)) error {
	err := this.db.View(func(txn *badger.Txn) error {
		iteratorOptions := badger.DefaultIteratorOptions
		iteratorOptions.Prefix = prefix // filter by prefix

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

func (this *prefixDb) ForEachWithPrefix(prefix []byte, consumer func([]byte, []byte)) error {
	return this.forEach(append(this.prefix, prefix...), consumer)
}

func (this *prefixDb) ForEach(consumer func([]byte, []byte)) error {
	return this.forEach(this.prefix, consumer)
}
