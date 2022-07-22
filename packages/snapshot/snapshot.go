package snapshot

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
)

type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
}

// CreateStreamableSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) CreateStreamableSnapshot(filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("fail to create snapshot file: %s", err)
	}

	outputProd := NewUTXOOutputProducer(t.Ledger)
	committableEC, fullEpochIndex, err := t.Options.CommitmentFunc()
	if err != nil {
		return err
	}

	err = StreamSnapshotDataTo(f, outputProd, fullEpochIndex, committableEC.EI(), committableEC, nmgr.SnapshotEpochDiffs)
	if err != nil {
		return err
	}
	f.Close()

	return err
}

// LoadStreamableSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) LoadStreamableSnapshot(filePath string,
	outputConsumer OutputConsumerFunc,
	epochDiffsConsumer EpochDiffsConsumerFunc,
	notarizationConsumer NotarizationConsumerFunc) error {
	if s.LedgerSnapshot == nil {
		s.LedgerSnapshot = new(ledger.Snapshot)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("fail to open snapshot file")
	}

	err = s.StreamSnapshotDataFrom(f, outputConsumer, epochDiffsConsumer, notarizationConsumer)
	if err != nil {
		return err
	}
	s.LedgerSnapshot.OutputWithMetadataCount = uint64(len(s.LedgerSnapshot.OutputsWithMetadata))
	f.Close()

	return err
}

func (s *Snapshot) FromNode(ledger *ledger.Ledger) {
	s.LedgerSnapshot = ledger.TakeSnapshot()
}

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

	marshaler.
		WriteUint64(s.LedgerSnapshot.OutputWithMetadataCount).
		WriteInt64(int64(s.LedgerSnapshot.FullEpochIndex)).
		WriteInt64(int64(s.LedgerSnapshot.DiffEpochIndex))

	data, err := serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot.LatestECRecord, serix.WithValidation())
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

// FromBytes returns a serialized version of the Snapshot.
func (s *Snapshot) FromBytes(data []byte) (err error) {
	s.LedgerSnapshot = new(ledger.Snapshot)
	reader := bytes.NewReader(data)

	outputConsumer := func(outputWithMetadatas []*ledger.OutputWithMetadata) {
		s.LedgerSnapshot.OutputsWithMetadata = append(s.LedgerSnapshot.OutputsWithMetadata, outputWithMetadatas...)
	}
	epochDiffsConsumer := func(fullEpochIndex, diffEpochIndex epoch.Index, epochDiffs map[epoch.Index]*ledger.EpochDiff) error {
		s.LedgerSnapshot.EpochDiffs = epochDiffs
		return nil
	}
	notarizationConsumer := func(fullEpochIndex, diffEpochIndex epoch.Index, latestECRecord *epoch.ECRecord) {}

	s.StreamSnapshotDataFrom(reader, outputConsumer, epochDiffsConsumer, notarizationConsumer)

	return nil
}

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

type OutputProducerFunc func() (outputWithMetadata *ledger.OutputWithMetadata)

type OutputConsumerFunc func(outputWithMetadatas []*ledger.OutputWithMetadata)

type EpochDiffProducerFunc func() (epochDiffs map[epoch.Index]*ledger.EpochDiff, err error)

type EpochDiffsConsumerFunc func(fullEpochIndex, diffEpochIndex epoch.Index, epochDiffs map[epoch.Index]*ledger.EpochDiff) error

type NotarizationConsumerFunc func(fullEpochIndex, diffEpochIndex epoch.Index, latestECRecord *epoch.ECRecord)
