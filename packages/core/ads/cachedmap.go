package ads

import (
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/typeutils"
)

type CachedMap[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]] struct {
	storedMap *Map[K, V, KPtr, VPtr]

	writeCache *shrinkingmap.ShrinkingMap[string, VPtr]
	readCache  *lru.Cache[string, VPtr]

	mutex           sync.RWMutex
	elementsToWrite *sync.Cond
}

func NewCachedMap[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]](store kvstore.KVStore, cacheSize int) (newMap *CachedMap[K, V, KPtr, VPtr]) {
	newMap = &CachedMap[K, V, KPtr, VPtr]{
		storedMap:  NewMap[K, V, KPtr, VPtr](store),
		writeCache: shrinkingmap.New[string, VPtr](),
		readCache:  lo.PanicOnErr(lru.New[string, VPtr](cacheSize)),
	}

	newMap.elementsToWrite = &sync.Cond{
		L: &newMap.mutex,
	}

	return
}

func (c *CachedMap[K, V, KPtr, VPtr]) Set(key K, value VPtr) {
	keyString := typeutils.BytesToString(lo.PanicOnErr(key.Bytes()))

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.writeCache.Set(keyString, value) {
		// increase pending writes counter
	}

	c.readCache.Remove(keyString)
}

func (c *CachedMap[K, V, KPtr, VPtr]) Delete(key K) {
	keyString := typeutils.BytesToString(lo.PanicOnErr(key.Bytes()))

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.writeCache.Set(keyString, nil)
	c.readCache.Remove(keyString)
}

func (c *CachedMap[K, V, KPtr, VPtr]) Has(key K) (has bool) {
	return lo.Return1(c.Get(key)) != nil
}

func (c *CachedMap[K, V, KPtr, VPtr]) Get(key K) (value VPtr, exists bool) {
	keyBytes := lo.PanicOnErr(key.Bytes())
	keyString := typeutils.BytesToString(keyBytes)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if writtenValue, exists := c.writeCache.Get(keyString); exists {
		return writtenValue, writtenValue != nil
	}

	if readValue, exists := c.readCache.Get(keyString); exists {
		return readValue, readValue != nil
	}

	value, exists = c.storedMap.Get(key)
	c.readCache.Add(keyString, value)

	return
}

func (c *CachedMap[K, V, KPtr, VPtr]) write() {
	for {
		c.mutex.Lock()
		for c.writeCache.Size() == 0 {
			c.elementsToWrite.Wait()
		}

		keyToWrite, valueToWrite := c.popFromWriteCache()
		c.mutex.Unlock()

		if valueToWrite == nil {
			c.storedMap.delete(typeutils.StringToBytes(keyToWrite))
		} else {
			c.storedMap.set(typeutils.StringToBytes(keyToWrite), lo.PanicOnErr(valueToWrite.Bytes()))
		}
	}
}

func (c *CachedMap[K, V, KPtr, VPtr]) popFromWriteCache() (key string, value VPtr) {
	c.writeCache.ForEach(func(randomKey string, randomValue VPtr) bool {
		key = randomKey
		value = randomValue

		c.writeCache.Delete(key)

		return false
	})

	return
}
