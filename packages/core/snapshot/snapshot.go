package snapshot

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// Snapshot contains the data to be put in a snapshot file.
type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
}

// CreateSnapshot creates a snapshot file to the given file path.
func CreateSnapshot(filePath string, t *tangleold.Tangle, nmgr *notarization.Manager) (*ledger.SnapshotHeader, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("fail to create snapshot file: %s", err)
	}

	lastConfirmedEpoch, err := nmgr.LatestConfirmedEpochIndex()
	if err != nil {
		return nil, err
	}
	outputWithMetadataProd := NewLedgerOutputWithMetadataProducer(lastConfirmedEpoch, t.Ledger)
	committableEC, fullEpochIndex, err := t.Options.CommitmentFunc()
	if err != nil {
		return nil, err
	}

	header := &ledger.SnapshotHeader{
		FullEpochIndex: fullEpochIndex,
		DiffEpochIndex: committableEC.EI(),
		LatestECRecord: committableEC,
	}

	err = StreamSnapshotDataTo(f, header, outputWithMetadataProd, nmgr.SnapshotEpochDiffs)
	if err != nil {
		return nil, err
	}
	f.Close()

	return header, err
}

// LoadSnapshot loads a snapshot file from the given file path. Contents in a snapshot file
// will not be written to a snapshot struct in case blowing up the memory, they should be proccessed in
// consumer functions. To construct a snapshot struct from a file, use FromBytes([]byte).
func LoadSnapshot(filePath string,
	outputWithMetadataConsumer OutputWithMetadataConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc,
	notarizationConsumer NotarizationConsumerFunc) (err error) {

	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		return fmt.Errorf("fail to open the snapshot file")
	}

	err = StreamSnapshotDataFrom(f, outputWithMetadataConsumer, epochDiffsConsumer, notarizationConsumer)

	return
}

// WriteFile a snapshot struct in to a file with a given file name.
func (s *Snapshot) WriteFile(fileName string) (err error) {
	data, err := s.Bytes()
	if err != nil {
		return err
	}

	if err = os.WriteFile(fileName, data, 0o644); err != nil {
		return errors.Errorf("failed to write snapshot file %s: %w", fileName, err)
	}

	return nil
}

// Bytes returns a serialized version of the Snapshot.
func (s *Snapshot) Bytes() (serialized []byte, err error) {
	marshaler := marshalutil.New()
	header := s.LedgerSnapshot.Header

	marshaler.
		WriteUint64(header.OutputWithMetadataCount).
		WriteInt64(int64(header.FullEpochIndex)).
		WriteInt64(int64(header.DiffEpochIndex))

	data, err := header.LatestECRecord.Bytes()
	if err != nil {
		return nil, err
	}
	marshaler.WriteBytes(data).WriteBytes(delimiter)

	// write outputWithMetadata
	typeSet := new(serix.TypeSettings)
	data, err = serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot.OutputsWithMetadata, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
	if err != nil {
		return nil, err
	}
	marshaler.WriteBytes(data).WriteBytes(delimiter)

	// write epochDiffs
	data, err = serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot.EpochDiffs, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
	if err != nil {
		return nil, err
	}
	marshaler.WriteBytes(data).WriteBytes(delimiter)

	return marshaler.Bytes(), nil
}

// FromBytes returns a Snapshot struct.
func (s *Snapshot) FromBytes(data []byte) (err error) {
	header := new(ledger.SnapshotHeader)
	ledgerSnapshot := new(ledger.Snapshot)

	reader := bytes.NewReader(data)

	outputWithMetadataConsumer := func(outputWithMetadatas []*ledger.OutputWithMetadata) {
		ledgerSnapshot.OutputsWithMetadata = append(ledgerSnapshot.OutputsWithMetadata, outputWithMetadatas...)
	}
	epochDiffsConsumer := func(_ *ledger.SnapshotHeader, epochDiffs map[epoch.Index]*ledger.EpochDiff) {
		ledgerSnapshot.EpochDiffs = epochDiffs
	}
	notarizationConsumer := func(h *ledger.SnapshotHeader) {
		header = h
	}

	err = StreamSnapshotDataFrom(reader, outputWithMetadataConsumer, epochDiffsConsumer, notarizationConsumer)
	header.OutputWithMetadataCount = uint64(len(ledgerSnapshot.OutputsWithMetadata))

	ledgerSnapshot.Header = header
	s.LedgerSnapshot = ledgerSnapshot
	return
}

// String returns a human readable snapshot.
func (s *Snapshot) String() (humanReadable string) {
	return stringify.Struct("Snapshot",
		stringify.StructField("LedgerSnapshot", s.LedgerSnapshot),
	)
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

// OutputWithMetadataProducerFunc is the type of function that produces OutputWithMetadatas when taking a snapshot.
type OutputWithMetadataProducerFunc func() (outputWithMetadata *ledger.OutputWithMetadata)

// OutputWithMetadataConsumerFunc is the type of function that consumes OutputWithMetadatas when loading a snapshot.
type OutputWithMetadataConsumerFunc func(outputWithMetadatas []*ledger.OutputWithMetadata)

// EpochDiffProducerFunc is the type of function that produces EpochDiff when taking a snapshot.
type EpochDiffProducerFunc func() (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error)

// EpochDiffsConsumerFunc is the type of function that consumes EpochDiff when loading a snapshot.
type EpochDiffsConsumerFunc func(header *ledger.SnapshotHeader, epochDiffs map[epoch.Index]*ledger.EpochDiff)

// NotarizationConsumerFunc is the type of function that consumes ECRecord when loading a snapshot.
type NotarizationConsumerFunc func(header *ledger.SnapshotHeader)
