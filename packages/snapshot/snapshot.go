package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
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
)

type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
}

// CreateStreamableSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) CreateStreamableSnapshot(filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("fail to create snapshot file")
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
func (s *Snapshot) LoadStreamableSnapshot(filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
	if s.LedgerSnapshot == nil {
		s.LedgerSnapshot = new(ledger.Snapshot)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("fail to open snapshot file")
	}

	outputConsumer := func(outputsWithMetadatas []*ledger.OutputWithMetadata) {
		t.Ledger.LoadOutputWithMetadatas(outputsWithMetadatas)
		nmgr.LoadOutputWithMetadatas(outputsWithMetadatas)
		s.LedgerSnapshot.OutputsWithMetadata = append(s.LedgerSnapshot.OutputsWithMetadata, outputsWithMetadatas...)
	}

	epochConsumer := func(fullEpochIndex epoch.Index, diffEpochIndex epoch.Index, epochDiffs map[epoch.Index]*ledger.EpochDiff) error {
		err := t.Ledger.LoadEpochDiffs(fullEpochIndex, diffEpochIndex, epochDiffs)
		if err != nil {
			return err
		}

		nmgr.LoadEpochDiffs(fullEpochIndex, diffEpochIndex, epochDiffs)
		s.LedgerSnapshot.EpochDiffs = epochDiffs
		return nil
	}

	err = StreamSnapshotDataFrom(f, outputConsumer, epochConsumer, nmgr.LoadECandEIs)
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

	// write epochDiffs
	typeSet := new(serix.TypeSettings)
	data, err = serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot.EpochDiffs, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
	if err != nil {
		return nil, err
	}
	marshaler.WriteBytes(data).WriteBytes(delimiter)

	// write outputWithMetadata
	data, err = serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot.OutputsWithMetadata, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
	if err != nil {
		return nil, err
	}
	marshaler.WriteBytes(data).WriteBytes(delimiter)

	// TODO: debug session, the index of outputID is not deserialized correctly
	outputMetadatas := make([]*ledger.OutputWithMetadata, 0)
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &outputMetadatas, serix.WithTypeSettings(typeSet.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)), serix.WithValidation())
	if err != nil {
		return nil, err
	}
	fmt.Println(outputMetadatas)

	return marshaler.Bytes(), nil
}

// FromBytes returns a serialized version of the Snapshot.
func (s *Snapshot) FromBytes(data []byte) (err error) {
	s.LedgerSnapshot = new(ledger.Snapshot)
	reader := bytes.NewReader(data)

	if err := binary.Read(reader, binary.LittleEndian, &s.LedgerSnapshot.OutputWithMetadataCount); err != nil {
		return fmt.Errorf("unable to read outputWithMetadata length: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &s.LedgerSnapshot.FullEpochIndex); err != nil {
		return fmt.Errorf("unable to read fullEpochIndex: %w", err)
	}

	if err := binary.Read(reader, binary.LittleEndian, &s.LedgerSnapshot.DiffEpochIndex); err != nil {
		return fmt.Errorf("unable to read diffEpochIndex: %w", err)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Split(scanDelimiter)

	// read LatestECRecord
	s.LedgerSnapshot.LatestECRecord, err = ReadECRecord(scanner)
	if err != nil {
		return err
	}

	// read epochDiffs
	s.LedgerSnapshot.EpochDiffs, err = ReadEpochDiffs(scanner)
	if err != nil {
		return err
	}

	outputs, err := ReadOutputWithMetadata(scanner)
	if err != nil {
		return err
	}
	s.LedgerSnapshot.OutputsWithMetadata = outputs

	// check if the file is consumed

	return nil
}

// func (s *Snapshot) String() (humanReadable string) {
// 	return stringify.Struct("Snapshot",
// 		stringify.StructField("LedgerSnapshot", s.LedgerSnapshot),
// 	)
// }

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
