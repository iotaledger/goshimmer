package peerregister

import (
	"sync"

	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

// PeerMap is the mapping of peer identifier and their peer struct
// It uses a mutex to handle concurrent access to its internal map
type PeerMap struct {
	sync.RWMutex
	internal map[string]*peer.Peer
}

// NewPeerMap returns a new PeerMap
func NewPeerMap() *PeerMap {
	return &PeerMap{
		internal: make(map[string]*peer.Peer),
	}
}

// Len returns the number of peers stored in a PeerMap
func (pm *PeerMap) Len() int {
	pm.RLock()
	defer pm.RUnlock()
	return len(pm.internal)
}

// GetMap returns the content of the entire internal map
func (pm *PeerMap) GetMap() map[string]*peer.Peer {
	newMap := make(map[string]*peer.Peer)
	pm.RLock()
	defer pm.RUnlock()
	for k, v := range pm.internal {
		newMap[k] = v
	}
	return newMap
}

// GetMap returns the content of the entire internal map
func (pm *PeerMap) GetSlice() []*peer.Peer {
	newSlice := make([]*peer.Peer, pm.Len())
	pm.RLock()
	defer pm.RUnlock()
	i := 0
	for _, v := range pm.internal {
		newSlice[i] = v
		i++
	}
	return newSlice
}

// Load returns the peer for a given key.
// It also return a bool to communicate the presence of the given
// peer into the internal map
func (pm *PeerMap) Load(key string) (value *peer.Peer, ok bool) {
	pm.RLock()
	defer pm.RUnlock()
	result, ok := pm.internal[key]
	return result, ok
}

// Delete removes the entire entry for a given key and return true if successful
func (pm *PeerMap) Delete(key string) (deletedPeer *peer.Peer, ok bool) {
	deletedPeer, ok = pm.Load(key)
	if !ok {
		return nil, false
	}
	pm.Lock()
	defer pm.Unlock()
	delete(pm.internal, key)
	return deletedPeer, true
}

// Store adds a new peer to the PeerMap
func (pm *PeerMap) Store(key string, value *peer.Peer) {
	pm.Lock()
	defer pm.Unlock()
	pm.internal[key] = value
}
