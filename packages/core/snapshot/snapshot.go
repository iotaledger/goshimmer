package snapshot

import (
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/mana"
)

// Snapshot contains the data to be put in a snapshot file.
type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
}

// CreateSnapshot creates a snapshot file to the given file path.
func CreateSnapshot(filePath string,
	headerProd HeaderProducerFunc,
	utxoStatesProd UTXOStatesProducerFunc,
	epochDiffsProd EpochDiffProducerFunc) (*ledger.SnapshotHeader, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("fail to create snapshot file: %s", err)
	}

	header, err := streamSnapshotDataTo(f, headerProd, utxoStatesProd, epochDiffsProd)
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
	epochDiffsConsumer EpochDiffsConsumerFunc) (err error) {

	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("fail to open the snapshot file")
	}

	err = streamSnapshotDataFrom(f, headerConsumer, outputWithMetadataConsumer, epochDiffsConsumer)

	return
}

func (s *Snapshot) updateConsensusManaDetails(nodeSnapshot *mana.SnapshotNode, output devnetvm.Output, outputMetadata *ledger.OutputMetadata) {
	pledgedValue := float64(0)
	output.Balances().ForEach(func(color devnetvm.Color, balance uint64) bool {
		pledgedValue += float64(balance)
		return true
	})

	nodeSnapshot.SortedTxSnapshot = append(nodeSnapshot.SortedTxSnapshot, &mana.TxSnapshot{
		Value:     pledgedValue,
		TxID:      output.ID().TransactionID,
		Timestamp: outputMetadata.CreationTime(),
	})
}

// UTXOStatesProducerFunc is the type of function that produces OutputWithMetadatas when taking a snapshot.
type UTXOStatesProducerFunc func() (outputWithMetadata *ledger.OutputWithMetadata)

// UTXOStatesConsumerFunc is the type of function that consumes OutputWithMetadatas when loading a snapshot.
type UTXOStatesConsumerFunc func(outputWithMetadatas []*ledger.OutputWithMetadata)

// EpochDiffProducerFunc is the type of function that produces EpochDiff when taking a snapshot.
type EpochDiffProducerFunc func() (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error)

// EpochDiffsConsumerFunc is the type of function that consumes EpochDiff when loading a snapshot.
type EpochDiffsConsumerFunc func(header *ledger.SnapshotHeader, epochDiffs map[epoch.Index]*ledger.EpochDiff)

// HeaderProducerFunc is the type of function that produces snapshot header when taking a snapshot.
type HeaderProducerFunc func() (header *ledger.SnapshotHeader, err error)

// HeaderConsumerFunc is the type of function that consumes ECRecord when loading a snapshot.
type HeaderConsumerFunc func(header *ledger.SnapshotHeader)
