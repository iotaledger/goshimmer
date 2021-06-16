package database

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

type CacheTimeProvider struct {
	forceCacheTime time.Duration
}

func NewCacheTimeProvider(forceCacheTime time.Duration) *CacheTimeProvider {
	return &CacheTimeProvider{forceCacheTime: forceCacheTime}
}

// CacheTime returns a CacheTime option. Duration may be overridden if ForceCacheTime parameter is a non-negative integer
func (m *CacheTimeProvider) CacheTime(duration time.Duration) objectstorage.Option {
	if m.forceCacheTime >= 0 {
		duration = m.forceCacheTime
	}
	return objectstorage.CacheTime(duration)
}
