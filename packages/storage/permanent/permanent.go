package permanent

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
)

const (
	unspentOutputsPrefix byte = iota
	unspentOutputIDsPrefix
	consensusWeightsPrefix
	attestationsPrefix
	throughputQuotaPrefix
)

type Permanent struct {
	Settings       *Settings
	Commitments    *Commitments
	UnspentOutputs kvstore.KVStore

	unspentOutputIDs kvstore.KVStore
	attestations     kvstore.KVStore
	sybilProtection  kvstore.KVStore
	throughputQuota  kvstore.KVStore
}

func New(disk *diskutil.DiskUtil, database *database.Manager) (p *Permanent) {
	return &Permanent{
		Settings:       NewSettings(disk.Path("settings.bin")),
		Commitments:    NewCommitments(disk.Path("commitments.bin")),
		UnspentOutputs: lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{unspentOutputsPrefix})),

		unspentOutputIDs: lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{unspentOutputIDsPrefix})),
		attestations:     lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{attestationsPrefix})),
		sybilProtection:  lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{consensusWeightsPrefix})),
		throughputQuota:  lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{throughputQuotaPrefix})),
	}
}

// UnspentOutputIDs returns the "unspent outputs ids" storage (or a specialized sub-storage if a realm is provided).
func (p *Permanent) UnspentOutputIDs(optRealm ...byte) kvstore.KVStore {
	if len(optRealm) == 0 {
		return p.unspentOutputIDs
	}

	return lo.PanicOnErr(p.unspentOutputIDs.WithExtendedRealm(optRealm))
}

// Attestations returns the "attestations" storage (or a specialized sub-storage if a realm is provided).
func (p *Permanent) Attestations(optRealm ...byte) kvstore.KVStore {
	if len(optRealm) == 0 {
		return p.attestations
	}

	return lo.PanicOnErr(p.attestations.WithExtendedRealm(optRealm))
}

// SybilProtection returns the sybil protection storage (or a specialized sub-storage if a realm is provided).
func (p *Permanent) SybilProtection(optRealm ...byte) kvstore.KVStore {
	if len(optRealm) == 0 {
		return p.sybilProtection
	}

	return lo.PanicOnErr(p.sybilProtection.WithExtendedRealm(optRealm))
}

// ThroughputQuota returns the throughput quota storage (or a specialized sub-storage if a realm is provided).
func (p *Permanent) ThroughputQuota(optRealm ...byte) kvstore.KVStore {
	if len(optRealm) == 0 {
		return p.throughputQuota
	}

	return lo.PanicOnErr(p.throughputQuota.WithExtendedRealm(optRealm))
}
