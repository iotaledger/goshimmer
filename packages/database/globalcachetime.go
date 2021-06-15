package database

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

type CacheTimeManager struct {
	forceCacheTime time.Duration
}

func NewCacheTimeManager(forceCacheTime time.Duration) *CacheTimeManager {
	return &CacheTimeManager{forceCacheTime: forceCacheTime}
}

// CacheTime returns a CacheTime option. Duration may be overridden if GlobalCacheTime parameter is a non-negative integer
func (m *CacheTimeManager) CacheTime(duration time.Duration) objectstorage.Option {
	if m.forceCacheTime >= 0 {
		duration = m.forceCacheTime
	}
	return objectstorage.CacheTime(duration)
}
