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

	mutex           sync.Mutex
	newElements     *sync.Cond
	writeCacheEmpty *sync.Cond
}

func NewCachedMap[K, V constraints.Serializable, KPtr constraints.MarshalablePtr[K], VPtr constraints.MarshalablePtr[V]](store kvstore.KVStore, cacheSize int) (newMap *CachedMap[K, V, KPtr, VPtr]) {
	newMap = &CachedMap[K, V, KPtr, VPtr]{
		storedMap:  NewMap[K, V, KPtr, VPtr](store),
		writeCache: shrinkingmap.New[string, VPtr](),
		readCache:  lo.PanicOnErr(lru.New[string, VPtr](cacheSize)),
	}

	newMap.newElements = &sync.Cond{
		L: &newMap.mutex,
	}

	newMap.writeCacheEmpty = &sync.Cond{
		L: &newMap.mutex,
	}

	go newMap.writeLoop()

	return
}

func (c *CachedMap[K, V, KPtr, VPtr]) Set(key K, value VPtr) {
	keyString := typeutils.BytesToString(lo.PanicOnErr(key.Bytes()))

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.writeCache.Set(keyString, value) && c.writeCache.Size() == 1 {
		c.newElements.Signal()
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

func (c *CachedMap[K, V, KPtr, VPtr]) writeLoop() {
	for {
		c.mutex.Lock()
		for c.writeCache.Size() == 0 {
			c.writeCacheEmpty.Broadcast()

			c.newElements.Wait()
		}

		keyToWrite, valueToWrite, exists := c.writeCache.Pop()
		if !exists {
			panic("writeCache should not be empty")
		}

		c.readCache.Add(keyToWrite, valueToWrite)
		c.mutex.Unlock()

		if valueToWrite == nil {
			c.storedMap.delete(typeutils.StringToBytes(keyToWrite))
		} else {
			c.storedMap.set(typeutils.StringToBytes(keyToWrite), lo.PanicOnErr(valueToWrite.Bytes()))
		}
	}
}
