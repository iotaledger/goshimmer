// Package mapdb provides a map implementation of a key value database.
// It offers a lightweight drop-in replacement of  hive.go/database for tests or in simulations
// where more than one instance is required.
package mapdb

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/hive.go/typeutils"
)

// MapDB is a simple implementation of DB using a map.
type MapDB struct {
	mu sync.RWMutex
	m  map[string]mapEntry
}

type mapEntry struct {
	value []byte
	meta  byte
}

// NewMapDB creates a database.Database implementation purely based on a go map.
// MapDB does not support TTL.
func NewMapDB() *MapDB {
	return &MapDB{
		m: make(map[string]mapEntry),
	}
}

func (db *MapDB) Contains(key database.Key) (contains bool, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	_, contains = db.m[typeutils.BytesToString(key)]
	return
}

func (db *MapDB) Get(key database.Key) (entry database.Entry, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	ent, contains := db.m[typeutils.BytesToString(key)]
	if !contains {
		err = database.ErrKeyNotFound
		return
	}
	entry.Key = key
	entry.Value = append([]byte{}, ent.value...)
	entry.Meta = ent.meta
	return
}

func (db *MapDB) GetKeyOnly(key database.Key) (entry database.KeyOnlyEntry, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	ent, contains := db.m[typeutils.BytesToString(key)]
	if !contains {
		err = database.ErrKeyNotFound
		return
	}
	entry.Key = key
	entry.Meta = ent.meta
	return
}

func (db *MapDB) Set(entry database.Entry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.m[typeutils.BytesToString(entry.Key)] = mapEntry{
		value: append([]byte{}, entry.Value...),
		meta:  entry.Meta,
	}
	return nil
}

func (db *MapDB) Delete(key database.Key) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.m, typeutils.BytesToString(key))
	return nil
}

func (db *MapDB) DeletePrefix(keyPrefix database.KeyPrefix) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	prefix := typeutils.BytesToString(keyPrefix)
	for key := range db.m {
		if strings.HasPrefix(key, prefix) {
			delete(db.m, key)
		}
	}
	return nil
}

func (db *MapDB) ForEach(consume func(entry database.Entry) bool) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for key, ent := range db.m {
		entry := database.Entry{
			Key:   []byte(key),
			Value: append([]byte{}, ent.value...),
			Meta:  ent.meta,
		}
		if consume(entry) {
			break
		}
	}
	return nil
}

func (db *MapDB) ForEachKeyOnly(consume func(entry database.KeyOnlyEntry) bool) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for key, ent := range db.m {
		entry := database.KeyOnlyEntry{
			Key:  []byte(key),
			Meta: ent.meta,
		}
		if consume(entry) {
			break
		}
	}
	return nil
}

func (db *MapDB) ForEachPrefix(keyPrefix database.KeyPrefix, consume func(entry database.Entry) (stop bool)) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	prefix := typeutils.BytesToString(keyPrefix)
	for key, ent := range db.m {
		if strings.HasPrefix(key, prefix) {
			entry := database.Entry{
				Key:   []byte(strings.TrimPrefix(key, prefix)),
				Value: append([]byte{}, ent.value...),
				Meta:  ent.meta,
			}
			if consume(entry) {
				break
			}
		}
	}
	return nil
}

func (db *MapDB) ForEachPrefixKeyOnly(keyPrefix database.KeyPrefix, consume func(entry database.KeyOnlyEntry) (stop bool)) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	prefix := typeutils.BytesToString(keyPrefix)
	for key, ent := range db.m {
		if strings.HasPrefix(key, prefix) {
			entry := database.KeyOnlyEntry{
				Key:  []byte(strings.TrimPrefix(key, prefix)),
				Meta: ent.meta,
			}
			if consume(entry) {
				break
			}
		}
	}
	return nil
}

func (db *MapDB) StreamForEach(consume func(entry database.Entry) error) (err error) {
	_ = db.ForEach(func(entry database.Entry) bool {
		err = consume(entry)
		return err != nil
	})
	return
}

func (db *MapDB) StreamForEachKeyOnly(consume func(entry database.KeyOnlyEntry) error) (err error) {
	_ = db.ForEachKeyOnly(func(entry database.KeyOnlyEntry) bool {
		err = consume(entry)
		return err != nil
	})
	return
}

func (db *MapDB) StreamForEachPrefix(keyPrefix database.KeyPrefix, consume func(entry database.Entry) error) (err error) {
	_ = db.ForEachPrefix(keyPrefix, func(entry database.Entry) bool {
		err = consume(entry)
		return err != nil
	})
	return
}

func (db *MapDB) StreamForEachPrefixKeyOnly(keyPrefix database.KeyPrefix, consume func(database.KeyOnlyEntry) error) (err error) {
	_ = db.ForEachPrefixKeyOnly(keyPrefix, func(entry database.KeyOnlyEntry) bool {
		err = consume(entry)
		return err != nil
	})
	return
}

func (db *MapDB) Apply(set []database.Entry, del []database.Key) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, entry := range set {
		db.m[typeutils.BytesToString(entry.Key)] = mapEntry{
			value: append([]byte{}, entry.Value...),
			meta:  entry.Meta,
		}
	}
	for _, key := range del {
		delete(db.m, typeutils.BytesToString(key))
	}
	return nil
}
