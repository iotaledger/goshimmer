package snapshot

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// Snapshot contains the data to be put in a snapshot file.
type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
}

// SolidEntryPoints contains solid entry points of an epoch.
type SolidEntryPoints struct {
	EI   epoch.Index         `serix:"0"`
	Seps []tangleold.BlockID `serix:"1,lengthPrefixType=uint32"`
}

func init() {
	typeSet := new(serix.TypeSettings)
	ts := typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)

	err := serix.DefaultAPI.RegisterTypeSettings([]*ledger.OutputWithMetadata{}, ts)
	if err != nil {
		panic(fmt.Errorf("error registering OutputWithMetadata slice type settings: %w", err))
	}

	err = serix.DefaultAPI.RegisterTypeSettings(map[epoch.Index]*ledger.EpochDiff{}, ts)
	if err != nil {
		panic(fmt.Errorf("error registering EpochDiff map type settings: %w", err))
	}
}

// CreateSnapshot creates a snapshot file to the given file path.
func CreateSnapshot(filePath string,
	headerProd HeaderProducerFunc,
	sepsProd SolidEntryPointsProducerFunc,
	utxoStatesProd UTXOStatesProducerFunc,
	epochDiffsProd EpochDiffProducerFunc) (*ledger.SnapshotHeader, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("fail to create snapshot file: %s", err)
	}

	header, err := streamSnapshotDataTo(f, headerProd, sepsProd, utxoStatesProd, epochDiffsProd)
	if err != nil {
		return nil, err
	}
	f.Close()

	return header, err
}

// LoadSnapshot loads a snapshot file from the given file path. Contents in a snapshot file
// will not be written to a snapshot struct in case blowing up the memory, they should be proccessed in
// consumer functions.
func LoadSnapshot(filePath string,
	headerConsumer HeaderConsumerFunc,
	sepsConsumer SolidEntryPointsConsumerFunc,
	outputWithMetadataConsumer UTXOStatesConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc) (err error) {

	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("fail to open the snapshot file")
	}

	err = streamSnapshotDataFrom(f, headerConsumer, sepsConsumer, outputWithMetadataConsumer, epochDiffsConsumer)

	return
}

// UTXOStatesProducerFunc is the type of function that produces OutputWithMetadatas when taking a snapshot.
type UTXOStatesProducerFunc func() (outputWithMetadata *ledger.OutputWithMetadata)

// UTXOStatesConsumerFunc is the type of function that consumes OutputWithMetadatas when loading a snapshot.
type UTXOStatesConsumerFunc func(outputWithMetadatas []*ledger.OutputWithMetadata)

// EpochDiffProducerFunc is the type of function that produces EpochDiff when taking a snapshot.
type EpochDiffProducerFunc func() (epochDiffs *ledger.EpochDiff)

// EpochDiffsConsumerFunc is the type of function that consumes EpochDiff when loading a snapshot.
type EpochDiffsConsumerFunc func(ei epoch.Index, epochDiffs *ledger.EpochDiff)

// HeaderProducerFunc is the type of function that produces snapshot header when taking a snapshot.
type HeaderProducerFunc func() (header *ledger.SnapshotHeader, err error)

// HeaderConsumerFunc is the type of function that consumes snapshot header when loading a snapshot.
type HeaderConsumerFunc func(header *ledger.SnapshotHeader)

// SolidEntryPointsProducerFunc is the type of function that produces solid entry points when taking a snapshot.
type SolidEntryPointsProducerFunc func() (seps *SolidEntryPoints)

// SolidEntryPointsConsumerFunc is the type of function that consumes solid entry points when loading a snapshot.
type SolidEntryPointsConsumerFunc func(seps *SolidEntryPoints)
