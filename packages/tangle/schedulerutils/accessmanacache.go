package schedulerutils

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
)

const (
	// manaCacheLifeTime time after which the mana vector should be refreshed.
	manaCacheLifeTime = 5 * time.Second
)

// AccessManaCache is a structure which provides access to cached access mana values.
// Mana values are refreshed after certain amount of time has passed.
type AccessManaCache struct {
	rawAccessManaVector        map[identity.ID]float64
	cacheRefreshTime           time.Time
	minMana                    float64
	accessManaMapRetrieverFunc func() map[identity.ID]float64
	mutex                      sync.RWMutex
}

// NewAccessManaCache returns a new AccessManaCache.
func NewAccessManaCache(accessManaMapRetrieverFunc func() map[identity.ID]float64, minMana float64) *AccessManaCache {
	accessManaCache := &AccessManaCache{
		minMana:                    minMana,
		accessManaMapRetrieverFunc: accessManaMapRetrieverFunc,
	}
	accessManaCache.refreshCacheIfNecessary()
	return accessManaCache
}

// GetCachedMana returns cached access mana value for a given node and refreshes mana vectors if they expired
// currently returns at least MinMana.
func (a *AccessManaCache) GetCachedMana(id identity.ID) float64 {
	a.refreshCacheIfNecessary()
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if mana, ok := a.rawAccessManaVector[id]; ok && mana >= a.minMana {
		return mana
	}
	// always return at least MinMana
	return a.minMana
}

// RawAccessManaVector returns raw access mana vector retrieved from mana plugin.
func (a *AccessManaCache) RawAccessManaVector() map[identity.ID]float64 {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.rawAccessManaVector
}

func (a *AccessManaCache) refreshCacheIfNecessary() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if time.Since(a.cacheRefreshTime) > manaCacheLifeTime {
		a.rawAccessManaVector = a.accessManaMapRetrieverFunc()
		a.cacheRefreshTime = time.Now()
	}
}
