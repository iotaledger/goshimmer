package sybilprotection

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Attestation struct {
	IssuerID         identity.ID       `serix:"0"`
	IssuingTime      time.Time         `serix:"1"`
	CommitmentID     commitment.ID     `serix:"2"`
	BlockContentHash types.Identifier  `serix:"3"`
	Signature        ed25519.Signature `serix:"4"`
}

func NewAttestation(issuerID identity.ID, issuingTime time.Time, commitmentID commitment.ID, blockContentHash types.Identifier, signature ed25519.Signature) *Attestation {
	return &Attestation{
		IssuerID:         issuerID,
		IssuingTime:      issuingTime,
		CommitmentID:     commitmentID,
		BlockContentHash: blockContentHash,
		Signature:        signature,
	}
}

func (a Attestation) Bytes() (bytes []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
}

func (a *Attestation) FromBytes(bytes []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, a, serix.WithValidation())
}

type EpochAttestations struct {
	weights *memstorage.Storage[identity.ID, int64]
	storage *memstorage.Storage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]]
}

func NewEpochAttestations(weights *memstorage.Storage[identity.ID, int64]) *EpochAttestations {
	return &EpochAttestations{
		weights: weights,
		storage: memstorage.New[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]](),
	}
}

func (a *EpochAttestations) TotalWeight() (totalWeight int64) {
	if a == nil {
		return 0
	}

	a.storage.ForEachKey(func(attestor identity.ID) bool {
		totalWeight += lo.Return1(a.weights.Get(attestor))

		return true
	})

	return
}

func (a *EpochAttestations) AuthenticatedSet() (adsAttestors *ads.Set[identity.ID]) {
	adsAttestors = ads.NewSet[identity.ID](mapdb.NewMapDB())

	if a == nil {
		return
	}

	a.storage.ForEachKey(func(attestor identity.ID) bool {
		adsAttestors.Add(attestor)

		return true
	})

	return
}

func (a *EpochAttestations) Add(block *models.Block) (added bool) {
	return lo.Return1(a.storage.RetrieveOrCreate(block.IssuerID(), memstorage.New[models.BlockID, *Attestation])).Set(
		block.ID(), NewAttestation(
			block.IssuerID(),
			block.IssuingTime(),
			block.Commitment().ID(),
			lo.PanicOnErr(block.ContentHash()),
			block.Signature(),
		),
	)
}

func (a *EpochAttestations) Delete(block *models.Block) (deleted bool) {
	if storage, exists := a.storage.Get(block.IssuerID()); exists {
		if deleted = storage.Delete(block.ID()); deleted && storage.IsEmpty() {
			a.storage.Delete(block.IssuerID())
		}
	}

	return
}
