package snapshot

import (
	"context"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/mana"
)

type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
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
