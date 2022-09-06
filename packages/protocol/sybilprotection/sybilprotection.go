package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type SybilProtection struct {
	ValidatorSet *validator.Set
}

func New() (sybilProtection *SybilProtection) {
	return &SybilProtection{
		ValidatorSet: validator.NewSet(),
	}
}
