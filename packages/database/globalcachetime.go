package database

import (
	"flag"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

var (
	globalCacheTime = -1
	once            sync.Once
)

// CacheTime returns a CacheTime option. Duration may be overridden if GlobalCacheTime parameter is a non-negative integer
func CacheTime(duration time.Duration) objectstorage.Option {
	// if test just disable cache
	if flag.Lookup("test.v") != nil {
		return objectstorage.CacheTime(0)
	}
	if globalCacheTime >= 0 {
		duration = time.Duration(globalCacheTime) * time.Second
	}
	return objectstorage.CacheTime(duration)
}

// SetGlobalCacheTimeOnce sets the global cache time in seconds. A negative number means no global cache time.
// This function should be called only once in the lifetime of the program
func SetGlobalCacheTimeOnce(cacheTime int) {
	once.Do(func() {
		globalCacheTime = cacheTime
	})
}
