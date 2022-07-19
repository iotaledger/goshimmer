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

// CreateSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) CreateSnapshot(filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("fail to create snapshot file")
	}

	outputProd := NewUTXOOutputProducer(t.Ledger)
	committableEC, fullEpochIndex, err := t.Options.CommitmentFunc()
	if err != nil {
		return err
	}

	err = StreamSnapshotDataTo(f, outputProd, fullEpochIndex, committableEC.EI(), nmgr.SnapshotEpochDiffs)
	if err != nil {
		return err
	}
	f.Close()

	return err
}

// LoadSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) LoadSnapshot(filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("fail to open snapshot file")
	}

	outputConsumer := func(outputsWithMetadatas []*ledger.OutputWithMetadata) {
		t.Ledger.LoadOutputWithMetadatas(outputsWithMetadatas)
		nmgr.LoadOutputWithMetadatas(outputsWithMetadatas)
		s.LedgerSnapshot.OutputsWithMetadata = append(s.LedgerSnapshot.OutputsWithMetadata, outputsWithMetadatas...)
	}
	s.LedgerSnapshot.OutputWithMetadataCount = uint64(len(s.LedgerSnapshot.OutputsWithMetadata))

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
		WriteInt64(int64(s.LedgerSnapshot.OutputWithMetadataCount)).
		WriteInt64(int64(s.LedgerSnapshot.FullEpochIndex)).
		WriteInt64(int64(s.LedgerSnapshot.DiffEpochIndex))

	// write epochDiffs
	data, err := serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot.EpochDiffs, serix.WithValidation())
	if err != nil {
		return nil, err
	}
	marshaler.WriteBytes(data).WriteByte(';')

	// write outputWithMetadata
	var outputChunkCounter int
	for _, output := range s.LedgerSnapshot.OutputsWithMetadata {
		outputChunkCounter++
		outputBytes, err := output.Bytes()
		if err != nil {
			return nil, fmt.Errorf("unable to serialize outputWithMetadata to bytes: %w", err)
		}

		marshaler.WriteBytes(outputBytes)

		// put a delimeter every 100 outputs
		if outputChunkCounter == 100 {
			marshaler.WriteByte(';')
		}
	}
	marshaler.WriteByte(';')

	return marshaler.Bytes(), nil
}

// FromBytes returns a serialized version of the Snapshot.
func (s *Snapshot) FromBytes(data []byte) (err error) {
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

	s.LedgerSnapshot.LatestECRecord, err = ReadECRecord(reader)
	if err != nil {
		return err
	}

	chunkReader := bufio.NewReader(reader)
	epochDiffs := make(map[epoch.Index]*ledger.EpochDiff)
	data, err = chunkReader.ReadBytes(byte(';'))
	if err != nil {
		return err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data, epochDiffs)
	s.LedgerSnapshot.EpochDiffs = epochDiffs

	if err != nil {
		return errors.Errorf("failed to parse epochDiffs from bytes: %w", err)
	}

	for i := 0; uint64(i) < s.LedgerSnapshot.OutputWithMetadataCount; {
		outputs, err := ReadOutputWithMetadata(chunkReader)
		if err != nil {
			return err
		}
		i += len(outputs)
		s.LedgerSnapshot.OutputsWithMetadata = append(s.LedgerSnapshot.OutputsWithMetadata, outputs...)
	}

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
