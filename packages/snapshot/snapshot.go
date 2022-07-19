package snapshot

import (
	"context"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
}

// CreateSnapshot creates a full snapshot for the given target milestone index.
func (s *Snapshot) CreateSnapshot(ctx context.Context, filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
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
func (s *Snapshot) LoadSnapshot(ctx context.Context, filePath string, t *tangle.Tangle, nmgr *notarization.Manager) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("fail to open snapshot file")
	}

	outputConsumer := func(outputsWithMetadatas []*ledger.OutputWithMetadata) {
		t.Ledger.LoadOutputWithMetadatas(outputsWithMetadatas)
		nmgr.LoadOutputWithMetadatas(outputsWithMetadatas)
	}

	epochConsumer := func(fullEpochIndex epoch.Index, diffEpochIndex epoch.Index, epochDiffs map[epoch.Index]*ledger.EpochDiff) error {
		err := t.Ledger.LoadEpochDiffs(fullEpochIndex, diffEpochIndex, epochDiffs)
		if err != nil {
			return err
		}

		nmgr.LoadEpochDiffs(fullEpochIndex, diffEpochIndex, epochDiffs)
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

func (s *Snapshot) FromFile(fileName string) (err error) {
	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return errors.Errorf("failed to read file %s: %w", fileName, err)
	}

	if err = s.FromBytes(bytes); err != nil {
		return errors.Errorf("failed to unmarshal Snapshot: %w", err)
	}

	return nil
}

func (s *Snapshot) FromBytes(bytes []byte) (err error) {
	s.LedgerSnapshot = new(ledger.Snapshot)
	_, err = serix.DefaultAPI.Decode(context.Background(), bytes, s.LedgerSnapshot)
	if err != nil {
		return errors.Errorf("failed to read LedgerSnapshot: %w", err)
	}

	for _, output := range s.LedgerSnapshot.OutputsWithMetadata {
		output.SetID(output.M.OutputID)
		output.Output().SetID(output.M.OutputID)
	}

	for _, epochdiff := range s.LedgerSnapshot.EpochDiffs {
		for _, spentOutput := range epochdiff.Spent() {
			spentOutput.SetID(spentOutput.M.OutputID)
			spentOutput.Output().SetID(spentOutput.M.OutputID)
		}
		for _, createdOutput := range epochdiff.Created() {
			createdOutput.SetID(createdOutput.M.OutputID)
			createdOutput.Output().SetID(createdOutput.M.OutputID)
		}
	}

	return nil
}

func (s *Snapshot) WriteFile(fileName string) (err error) {
	if err = os.WriteFile(fileName, s.Bytes(), 0o644); err != nil {
		return errors.Errorf("failed to write snapshot file %s: %w", fileName, err)
	}

	return nil
}

// Bytes returns a serialized version of the Snapshot.
func (s *Snapshot) Bytes() (serialized []byte) {
	return marshalutil.New().
		WriteBytes(lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), s.LedgerSnapshot))).
		Bytes()
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
