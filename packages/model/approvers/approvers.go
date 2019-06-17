package approvers

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ternary"
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
