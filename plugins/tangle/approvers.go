package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ternary"
	"sync"
)

type Approvers struct {
	hash        ternary.Trinary
	hashes      map[ternary.Trinary]bool
	hashesMutex sync.RWMutex
	modified    bool
}

func NewApprovers(hash ternary.Trinary) *Approvers {
	return &Approvers{
		hash:     hash,
		hashes:   make(map[ternary.Trinary]bool),
		modified: false,
	}
}

// region public methods with locking //////////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) Add(transactionHash ternary.Trinary) {
	approvers.hashesMutex.Lock()
	defer approvers.hashesMutex.Unlock()

	approvers.add(transactionHash)
}

func (approvers *Approvers) Remove(approverHash ternary.Trinary) {
	approvers.hashesMutex.Lock()
	defer approvers.hashesMutex.Unlock()

	approvers.remove(approverHash)
}

func (approvers *Approvers) GetHashes() []ternary.Trinary {
	approvers.hashesMutex.RLock()
	defer approvers.hashesMutex.RUnlock()

	return approvers.getHashes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region private methods without locking //////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) add(transactionHash ternary.Trinary) {
	if _, exists := approvers.hashes[transactionHash]; !exists {
		approvers.hashes[transactionHash] = true
		approvers.modified = true
	}
}

func (approvers *Approvers) remove(approverHash ternary.Trinary) {
	if _, exists := approvers.hashes[approverHash]; exists {
		delete(approvers.hashes, approverHash)
		approvers.modified = true
	}
}

func (approvers *Approvers) getHashes() []ternary.Trinary {
	hashes := make([]ternary.Trinary, len(approvers.hashes))

	counter := 0
	for hash, _ := range approvers.hashes {
		hashes[counter] = hash

		counter++
	}

	return hashes
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) Store(approverHash ternary.Trinary) {
	approvers.hashesMutex.Lock()
	approvers.hashesMutex.RUnlock()

	approvers.modified = false
}

func (approvers *Approvers) Marshal() []byte {
	approvers.hashesMutex.RLock()
	defer approvers.hashesMutex.RUnlock()

	return make([]byte, 0)
}
