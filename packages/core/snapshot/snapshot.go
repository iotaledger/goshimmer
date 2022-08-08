package snapshot

import (
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/hive.go/core/serix"
)

// Snapshot contains the data to be put in a snapshot file.
type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
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

	err = serix.DefaultAPI.RegisterTypeSettings(epoch.SnapshotEpochActivity{}, ts)
	if err != nil {
		panic(fmt.Errorf("error registering EpochDiff map type settings: %w", err))
	}
}

// CreateSnapshot creates a snapshot file to the given file path.
func CreateSnapshot(filePath string, headerProd HeaderProducerFunc, utxoStatesProd UTXOStatesProducerFunc, epochDiffsProd EpochDiffProducerFunc, activityLogProd ActivityLogProducerFunc) (*ledger.SnapshotHeader, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("fail to create snapshot file: %s", err)
	}

	header, err := streamSnapshotDataTo(f, headerProd, utxoStatesProd, epochDiffsProd, activityLogProd)
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
	outputWithMetadataConsumer UTXOStatesConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc,
	activityLogConsumer ActivityLogConsumerFunc) (err error) {

	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("fail to open the snapshot file")
	}

	err = streamSnapshotDataFrom(f, headerConsumer, outputWithMetadataConsumer, epochDiffsConsumer, activityLogConsumer)

	return
}

// UTXOStatesProducerFunc is the type of function that produces OutputWithMetadatas when taking a snapshot.
type UTXOStatesProducerFunc func() (outputWithMetadata *ledger.OutputWithMetadata)

// UTXOStatesConsumerFunc is the type of function that consumes OutputWithMetadatas when loading a snapshot.
type UTXOStatesConsumerFunc func(outputWithMetadatas []*ledger.OutputWithMetadata)

// EpochDiffProducerFunc is the type of function that produces EpochDiff when taking a snapshot.
type EpochDiffProducerFunc func() (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error)

// EpochDiffsConsumerFunc is the type of function that consumes EpochDiff when loading a snapshot.
type EpochDiffsConsumerFunc func(header *ledger.SnapshotHeader, epochDiffs map[epoch.Index]*ledger.EpochDiff)

// ActivityLogProducerFunc is the type of function that produces ActivityLog when loading a snapshot.
type ActivityLogProducerFunc func() (activityLogs epoch.SnapshotEpochActivity)

// ActivityLogConsumerFunc is the type of function that consumes Activity logs when loading a snapshot.
type ActivityLogConsumerFunc func(activityLogs epoch.SnapshotEpochActivity)

// HeaderProducerFunc is the type of function that produces snapshot header when taking a snapshot.
type HeaderProducerFunc func() (header *ledger.SnapshotHeader, err error)

// HeaderConsumerFunc is the type of function that consumes snapshot header when loading a snapshot.
type HeaderConsumerFunc func(header *ledger.SnapshotHeader)
