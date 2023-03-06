package notarization

import (
	"bytes"
	"context"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

type Attestation struct {
	IssuerPublicKey  ed25519.PublicKey `serix:"0"`
	IssuingTime      time.Time         `serix:"1"`
	CommitmentID     commitment.ID     `serix:"2"`
	BlockContentHash types.Identifier  `serix:"3"`
	Signature        ed25519.Signature `serix:"4"`

	id models.BlockID
}

func NewAttestation(block *models.Block, slotTimeProvider *slot.TimeProvider) *Attestation {
	a := &Attestation{
		IssuerPublicKey:  block.IssuerPublicKey(),
		IssuingTime:      block.IssuingTime(),
		CommitmentID:     block.Commitment().ID(),
		BlockContentHash: lo.PanicOnErr(block.ContentHash()),
		Signature:        block.Signature(),
	}
	a.id = models.NewBlockID(a.BlockContentHash, a.Signature, slotTimeProvider.IndexFromTime(a.IssuingTime))

	return a
}

func (a *Attestation) Compare(other *Attestation) int {
	switch {
	case a == nil && other == nil:
		return 0
	case a == nil:
		return -1
	case other == nil:
		return 1
	case a.IssuingTime.After(other.IssuingTime):
		return 1
	case other.IssuingTime.After(a.IssuingTime):
		return -1
	default:
		return bytes.Compare(a.BlockContentHash[:], other.BlockContentHash[:])
	}
}

func (a *Attestation) ID() models.BlockID {
	return a.id
}

func (a Attestation) Bytes() (bytes []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
}

func (a *Attestation) FromBytes(bytes []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, a, serix.WithValidation())
}

func (a *Attestation) IssuerID() identity.ID {
	return identity.NewID(a.IssuerPublicKey)
}

func (a *Attestation) VerifySignature() (valid bool, err error) {
	issuingTimeBytes, err := serix.DefaultAPI.Encode(context.Background(), a.IssuingTime, serix.WithValidation())
	if err != nil {
		return false, err
	}

	if !a.IssuerPublicKey.VerifySignature(byteutils.ConcatBytes(issuingTimeBytes, lo.PanicOnErr(a.CommitmentID.Bytes()), a.BlockContentHash[:]), a.Signature) {
		return false, nil
	}

	return true, nil
}
