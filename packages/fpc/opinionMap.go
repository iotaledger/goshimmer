package fpc

import (
	"fmt"
	"sync"
)

// OpinionMap is the mapping of Txs and their voting value history throughout all FPC rounds.
// It uses a mutex to handle concurrent access to its internal map
type OpinionMap struct {
	sync.RWMutex
	internal map[ID]Opinions
}

// NewOpinionMap returns a new OpinionMap
func NewOpinionMap() *OpinionMap {
	return &OpinionMap{
		internal: make(map[ID]Opinions),
	}
}

// Len returns the number of txs stored in a new OpinionMap
func (rm *OpinionMap) Len() int {
	rm.RLock()
	defer rm.RUnlock()
	return len(rm.internal)
}

// GetMap returns the content of the entire internal map
func (rm *OpinionMap) GetMap() map[ID]Opinions {
	newMap := make(map[ID]Opinions)
	rm.RLock()
	defer rm.RUnlock()
	for k, v := range rm.internal {
		newMap[k] = v
	}
	return newMap
}

// Load returns the opinion for a given ID.
// It also return a bool to communicate the presence of the given
// ID into the internal map
func (rm *OpinionMap) Load(key ID) (value Opinions, ok bool) {
	rm.RLock()
	defer rm.RUnlock()
	result, ok := rm.internal[key]
	return result, ok
}

// Delete removes the entire entry for a given ID
func (rm *OpinionMap) Delete(key ID) {
	rm.Lock()
	defer rm.Unlock()
	delete(rm.internal, key)
}

// Store adds a new opinion to the history of a given ID
func (rm *OpinionMap) Store(key ID, value bool) {
	rm.Lock()
	defer rm.Unlock()
	rm.internal[key] = append(rm.internal[key], value)
}

// String returns the string rapresentation of OpinionMap
func (rm *OpinionMap) String() string {
	out := ""
	for k, elem := range rm.GetMap() {
		out += fmt.Sprintf("tx: %v %v, ", k, elem)
	}
	return out
}
