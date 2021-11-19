package schedulerutils

import (
	"fmt"
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
	updatedAccessManaVector    map[identity.ID]float64
	cacheRefreshTime           time.Time
	minMana                    float64
	accessManaMapRetrieverFunc func() map[identity.ID]float64
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

	if mana, ok := a.updatedAccessManaVector[id]; ok && mana >= a.minMana {
		return mana
	}
	// TODO: return actual mana value instead of MinMana
	// always return at least MinMana
	a.updatedAccessManaVector[id] = a.minMana
	return a.updatedAccessManaVector[id]
}

// RawAccessManaVector returns raw access mana vector retrieved from mana plugin.
func (a *AccessManaCache) RawAccessManaVector() map[identity.ID]float64 {
	return a.rawAccessManaVector
}

// UpdatedAccessManaVector returns raw updated mana vector retrieved from mana plugin with nodes added.
func (a *AccessManaCache) UpdatedAccessManaVector() map[identity.ID]float64 {
	return a.updatedAccessManaVector
}

func (a *AccessManaCache) refreshCacheIfNecessary() {
	if time.Since(a.cacheRefreshTime) > manaCacheLifeTime {
		fmt.Println("Refresh access mana")
		a.rawAccessManaVector = a.accessManaMapRetrieverFunc()
		a.updatedAccessManaVector = a.rawAccessManaVector
		a.cacheRefreshTime = time.Now()
	}
}
