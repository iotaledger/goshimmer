package fpc

import (
	"fmt"
	"sync"
)

// OpinionMap is the mapping of Txs and their voting value history throughout all FPC rounds.
// It uses a mutex to handle concurrent access to its internal map
type OpinionMap struct {
	sync.RWMutex
	internal map[Hash]Opinions
}

// NewOpinionMap returns a new OpinionMap
func NewOpinionMap() *OpinionMap {
	return &OpinionMap{
		internal: make(map[Hash]Opinions),
	}
}

// Len returns the number of txs stored in a new OpinionMap
func (rm *OpinionMap) Len() int {
	rm.RLock()
	defer rm.RUnlock()
	return len(rm.internal)
}

// GetMap returns the content of the entire internal map
func (rm *OpinionMap) GetMap() map[Hash][]Opinion {
	newMap := make(map[Hash][]Opinion)
	rm.RLock()
	defer rm.RUnlock()
	for k, v := range rm.internal {
		newMap[k] = v
	}
	return newMap
}

// Load returns the opinion of a given tx Hash.
// It also return a bool to communicate the presence of the given
// tx Hash into the internal map
func (rm *OpinionMap) Load(key Hash) (value []Opinion, ok bool) {
	rm.RLock()
	result, ok := rm.internal[key]
	rm.RUnlock()
	return result, ok
}

// Delete removes the entire entry for a given tx Hash
func (rm *OpinionMap) Delete(key Hash) {
	rm.Lock()
	delete(rm.internal, key)
	rm.Unlock()
}

// Store adds a new opinion to the history of a given tx Hash
func (rm *OpinionMap) Store(key Hash, value Opinion) {
	rm.Lock()
	rm.internal[key] = append(rm.internal[key], value)
	rm.Unlock()
}

// String returns the string rapresentation of OpinionMap
func (rm *OpinionMap) String() string {
	out := ""
	for k, elem := range rm.GetMap() {
		out += fmt.Sprintf("tx: %v %v, ", k, elem)
	}
	return out
}
