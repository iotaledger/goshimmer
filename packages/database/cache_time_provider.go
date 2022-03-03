package database

import (
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"
)

// CacheTimeProvider should be always used to get the CacheTime option for storage
// It wraps around objectstorage.CacheTime() function and may override the input duration.
type CacheTimeProvider struct {
	forceCacheTime time.Duration
}

// NewCacheTimeProvider creates an instance that forces cache time to always be a certain value.
// If the given value is negative, hard coded defaults will be used.
func NewCacheTimeProvider(forceCacheTime time.Duration) *CacheTimeProvider {
	return &CacheTimeProvider{forceCacheTime: forceCacheTime}
}

// CacheTime returns a CacheTime option. Duration may be overridden if CacheTimeProvider parameter is a non-negative integer.
func (m *CacheTimeProvider) CacheTime(duration time.Duration) objectstorage.Option {
	if m.forceCacheTime >= 0 {
		duration = m.forceCacheTime
	}
	return objectstorage.CacheTime(duration)
}
