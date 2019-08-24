package gossip

import (
	"sync"
)

// NeighborMap is the mapping of neighbor identifier and their neighbor struct
// It uses a mutex to handle concurrent access to its internal map
type NeighborMap struct {
	sync.RWMutex
	internal map[string]*Neighbor
}

// NewPeerMap returns a new PeerMap
func NewNeighborMap() *NeighborMap {
	return &NeighborMap{
		internal: make(map[string]*Neighbor),
	}
}

// Len returns the number of peers stored in a PeerMap
func (nm *NeighborMap) Len() int {
	nm.RLock()
	defer nm.RUnlock()
	return len(nm.internal)
}

// GetMap returns the content of the entire internal map
func (nm *NeighborMap) GetMap() map[string]*Neighbor {
	newMap := make(map[string]*Neighbor)
	nm.RLock()
	defer nm.RUnlock()
	for k, v := range nm.internal {
		newMap[k] = v
	}
	return newMap
}

// GetMap returns the content of the entire internal map
func (nm *NeighborMap) GetSlice() []*Neighbor {
	newSlice := make([]*Neighbor, nm.Len())
	nm.RLock()
	defer nm.RUnlock()
	i := 0
	for _, v := range nm.internal {
		newSlice[i] = v
		i++
	}
	return newSlice
}

// Load returns the peer for a given key.
// It also return a bool to communicate the presence of the given
// peer into the internal map
func (nm *NeighborMap) Load(key string) (value *Neighbor, ok bool) {
	nm.RLock()
	defer nm.RUnlock()
	result, ok := nm.internal[key]
	return result, ok
}

// Delete removes the entire entry for a given key and return true if successful
func (nm *NeighborMap) Delete(key string) (deletedPeer *Neighbor, ok bool) {
	deletedPeer, ok = nm.Load(key)
	if !ok {
		return nil, false
	}
	nm.Lock()
	defer nm.Unlock()
	delete(nm.internal, key)
	return deletedPeer, true
}

// Store adds a new peer to the PeerMap
func (nm *NeighborMap) Store(key string, value *Neighbor) {
	nm.Lock()
	defer nm.Unlock()
	nm.internal[key] = value
}
