package iota

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type Protocol struct {
	EvictionManager *eviction.Manager[models.BlockID]
	ValidatorSet    *validator.Set

	optsTangleOptions []options.Option[tangle.Tangle]

	*tangle.Tangle
	// Scheduler
	// Consensus
	// Notarization
	// Tip manager
	// Block factory
	// Mana
}

func NewProtocol(opts ...options.Option[Protocol]) (newProtocol *Protocol) {
	return options.Apply(&Protocol{}, opts, func(p *Protocol) {
		p.Tangle = tangle.New(p.EvictionManager, p.ValidatorSet, p.optsTangleOptions...)
	})
}
