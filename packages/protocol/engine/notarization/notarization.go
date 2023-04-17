package notarization

import (
	"io"

	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/module"
)

type Notarization interface {
	Events() *Events

	Attestations() Attestations

	// IsFullyCommitted returns if notarization finished committing all pending slots up to the current acceptance time.
	IsFullyCommitted() bool

	NotarizeAcceptedBlock(block *models.Block) (err error)

	NotarizeOrphanedBlock(block *models.Block) (err error)

	Import(reader io.ReadSeeker) (err error)

	Export(writer io.WriteSeeker, targetSlot slot.Index) (err error)

	PerformLocked(perform func(m Notarization))

	module.Interface
}

type Attestations interface {
	Get(index slot.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], err error)

	traits.Committable
	module.Interface
}
