package fcob

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ternary"
)

// VotingnMap is the mapping of Txs and their being voted status.
// It uses a mutex to handle concurrent access to its internal map
type VotingnMap struct {
	sync.RWMutex
	internal map[ternary.Trinary]bool
}

// NewVotingMap returns a new VotingnMap
func NewVotingMap() *VotingnMap {
	return &VotingnMap{
		internal: make(map[ternary.Trinary]bool),
	}
}

// Len returns the number of txs stored in the votingMap
func (vm *VotingnMap) Len() int {
	vm.RLock()
	defer vm.RUnlock()
	return len(vm.internal)
}

// GetMap returns the content of the entire internal map
func (vm *VotingnMap) GetMap() map[ternary.Trinary]bool {
	newMap := make(map[ternary.Trinary]bool)
	vm.RLock()
	defer vm.RUnlock()
	for k, v := range vm.internal {
		newMap[k] = v
	}
	return newMap
}

// Load returns the value for a given key.
// It returns false it the key is not present
func (vm *VotingnMap) Load(key ternary.Trinary) bool {
	vm.RLock()
	defer vm.RUnlock()
	return vm.internal[key]
}

// Delete removes the entire entry for a given key
func (vm *VotingnMap) Delete(key ternary.Trinary) {
	vm.Lock()
	defer vm.Unlock()
	delete(vm.internal, key)
}

// Store adds a new entries to the map
func (vm *VotingnMap) Store(keys ...ternary.Trinary) {
	vm.Lock()
	defer vm.Unlock()
	for _, key := range keys {
		vm.internal[key] = true
	}
}

// String returns the string rapresentation of VotingMap
func (vm *VotingnMap) String() string {
	out := ""
	for k := range vm.GetMap() {
		out += fmt.Sprintf("%v\n", k)
	}
	return out
}
