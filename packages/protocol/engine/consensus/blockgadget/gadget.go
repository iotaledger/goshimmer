package blockgadget

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Gadget interface {
	Events() *Events

	// IsBlockAccepted returns whether the given block is accepted.
	IsBlockAccepted(blockID models.BlockID) (accepted bool)

	IsBlockConfirmed(blockID models.BlockID) bool

	// IsMarkerAccepted returns whether the given marker is accepted.
	IsMarkerAccepted(marker markers.Marker) (accepted bool)

	FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index)

	module.Interface
}
