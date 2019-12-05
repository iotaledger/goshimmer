package neighbor

import (
	"sync"

	"github.com/capossele/gossip/transport"
	"github.com/iotaledger/autopeering-sim/peer"
)

// Neighbor defines a neighbor
type Neighbor struct {
	Peer *peer.Peer
	Conn *transport.Connection
}

// NeighborMap implements a map of neighbors thread safe
type NeighborMap struct {
	sync.RWMutex
	internal map[string]*Neighbor
}

// NewMap returns a new NeighborMap
func NewMap() *NeighborMap {
	return &NeighborMap{
		internal: make(map[string]*Neighbor),
	}
}

// New returns a new Neighbor
func New(peer *peer.Peer, conn *transport.Connection) *Neighbor {
	return &Neighbor{
		Peer: peer,
		Conn: conn,
	}
}

// Len returns the number of neighbors stored in a NeighborMap
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

// GetSlice returns a slice of the content of the entire internal map
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

// Load returns the neighbor for a given key.
// It also return a bool to communicate the presence of the given
// neighbor into the internal map
func (nm *NeighborMap) Load(key string) (value *Neighbor, ok bool) {
	nm.RLock()
	defer nm.RUnlock()
	result, ok := nm.internal[key]
	return result, ok
}

// Delete removes the entire entry for a given key and return true if successful
func (nm *NeighborMap) Delete(key string) (deletedNeighbor *Neighbor, ok bool) {
	deletedNeighbor, ok = nm.Load(key)
	if !ok {
		return nil, false
	}
	nm.Lock()
	defer nm.Unlock()
	if deletedNeighbor.Conn != nil {
		deletedNeighbor.Conn.Close()
	}
	delete(nm.internal, key)
	return deletedNeighbor, true
}

// Store adds a new neighbor to the NeighborMap
func (nm *NeighborMap) Store(key string, value *Neighbor) {
	nm.Lock()
	defer nm.Unlock()
	nm.internal[key] = value
}
