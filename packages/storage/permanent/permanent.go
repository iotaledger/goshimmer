package permanent

import (
	"os"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
)

const (
	unspentOutputsPrefix byte = iota
	unspentOutputIDsPrefix
	consensusWeightsPrefix
)

type Permanent struct {
	Settings         *Settings
	Commitments      *Commitments
	UnspentOutputs   *UnspentOutputs
	UnspentOutputIDs kvstore.KVStore
	SybilProtection  kvstore.KVStore
}

func New(disk *diskutil.DiskUtil, database *database.Manager) (p *Permanent) {
	return &Permanent{
		Settings:         NewSettings(disk.Path("settings.bin")),
		Commitments:      NewCommitments(disk.Path("commitments.bin")),
		UnspentOutputs:   NewUnspentOutputs(lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{unspentOutputsPrefix}))),
		UnspentOutputIDs: lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{unspentOutputIDsPrefix})),
		SybilProtection:  lo.PanicOnErr(database.PermanentStorage().WithRealm([]byte{consensusWeightsPrefix})),
	}
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
