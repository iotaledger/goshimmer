package permanent

import (
	"os"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
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

// New returns a new permanent storage instance.
func New(dir *utils.Directory, db *database.Manager) (p *Permanent) {
	return &Permanent{
		Settings:       NewSettings(dir.Path("settings.bin")),
		Commitments:    NewCommitments(dir.Path("commitments.bin")),
		UnspentOutputs: lo.PanicOnErr(db.PermanentStorage().WithExtendedRealm([]byte{unspentOutputsPrefix})),

		unspentOutputIDs: lo.PanicOnErr(db.PermanentStorage().WithExtendedRealm([]byte{unspentOutputIDsPrefix})),
		attestations:     lo.PanicOnErr(db.PermanentStorage().WithExtendedRealm([]byte{attestationsPrefix})),
		sybilProtection:  lo.PanicOnErr(db.PermanentStorage().WithExtendedRealm([]byte{consensusWeightsPrefix})),
		throughputQuota:  lo.PanicOnErr(db.PermanentStorage().WithExtendedRealm([]byte{throughputQuotaPrefix})),
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

// SettingsAndCommitmentsSize returns the total size of the binary files.
func (p *Permanent) SettingsAndCommitmentsSize() int64 {
	var sum int64

	files := []string{p.Settings.FilePath(), p.Commitments.FilePath()}
	for _, file := range files {
		size, err := fileSize(file)
		if err != nil {
			panic(err)
		}
		sum += size
	}

	return sum
}

func fileSize(path string) (int64, error) {
	s, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	return s.Size(), nil
}
