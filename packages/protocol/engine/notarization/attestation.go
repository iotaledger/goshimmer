package notarization

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Attestation struct {
	IssuerID         identity.ID       `serix:"0"`
	IssuingTime      time.Time         `serix:"1"`
	CommitmentID     commitment.ID     `serix:"2"`
	BlockContentHash types.Identifier  `serix:"3"`
	Signature        ed25519.Signature `serix:"4"`
}

func NewAttestation(block *models.Block) *Attestation {
	return &Attestation{
		block.IssuerID(),
		block.IssuingTime(),
		block.Commitment().ID(),
		lo.PanicOnErr(block.ContentHash()),
		block.Signature(),
	}
}

func (a Attestation) Bytes() (bytes []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
}

func (a *Attestation) FromBytes(bytes []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, a, serix.WithValidation())
}