package schedulerutils

import (
	"math"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/identity"
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
	totalMana                  float64
	accessManaMapRetrieverFunc func() map[identity.ID]float64
	totalManaRetrieverFunc     func() float64
	mutex                      sync.RWMutex
}

// NewAccessManaCache returns a new AccessManaCache.
func NewAccessManaCache(accessManaMapRetrieverFunc func() map[identity.ID]float64, totalAccessManaRetrieverFunc func() float64, minMana float64) *AccessManaCache {
	accessManaCache := &AccessManaCache{
		minMana:                    minMana,
		accessManaMapRetrieverFunc: accessManaMapRetrieverFunc,
		totalManaRetrieverFunc:     totalAccessManaRetrieverFunc,
	}
	return accessManaCache
}

// GetCachedMana returns cached access mana value for a given node and refreshes mana vectors if they expired
// currently returns at least MinMana.
func (a *AccessManaCache) GetCachedMana(id identity.ID) float64 {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if mana, ok := a.rawAccessManaVector[id]; ok && mana >= a.minMana {
		return mana
	}
	// always return at least MinMana
	return a.minMana
}

// GetCachedTotalMana returns cached total mana value.
func (a *AccessManaCache) GetCachedTotalMana() float64 {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return math.Max(1.0, a.totalMana)
}

// RawAccessManaVector returns raw access mana vector retrieved from mana plugin.
func (a *AccessManaCache) RawAccessManaVector() map[identity.ID]float64 {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.rawAccessManaVector
}

// RefreshCacheIfNecessary refreshes mana cache after it has expired.
func (a *AccessManaCache) RefreshCacheIfNecessary() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if time.Since(a.cacheRefreshTime) > manaCacheLifeTime {
		a.rawAccessManaVector = a.accessManaMapRetrieverFunc()
		a.totalMana = a.totalManaRetrieverFunc()
		a.cacheRefreshTime = time.Now()
	}
}
