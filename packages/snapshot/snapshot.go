package snapshot

import (
	"fmt"
	"os"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
)

type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
	ManaSnapshot   *mana.Snapshot
}

func (s *Snapshot) FromNode(ledger *ledger.Ledger, accessManaByNode mana.NodeMap, accessManaTime time.Time) {
	s.LedgerSnapshot = ledger.TakeSnapshot()
	s.ManaSnapshot = s.takeManaSnapshot(accessManaByNode, accessManaTime)
}

func (s *Snapshot) FromFile(fileName string) (err error) {
	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return errors.Errorf("failed to read file %s: %w", fileName, err)
	}

	if err = s.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to unmarshal Snapshot: %w", err)
	}

	return nil
}

func (s *Snapshot) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	s.LedgerSnapshot = ledger.NewSnapshot(utxo.NewOutputs(), ledger.NewOutputsMetadata())
	if err = s.LedgerSnapshot.FromMarshalUtil(marshalUtil, devnetvm.OutputFactory); err != nil {
		return errors.Errorf("failed to read LedgerSnapshot: %w", err)
	}

	s.ManaSnapshot = mana.NewSnapshot()
	if err = s.ManaSnapshot.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to read ManaSnapshot: %w", err)
	}

	return nil
}

func (s *Snapshot) WriteFile(fileName string) (err error) {
	if err = os.WriteFile(fileName, s.Bytes(), 0644); err != nil {
		return errors.Errorf("failed to write snapshot file %s: %w", fileName, err)
	}

	return nil
}

// Bytes returns a serialized version of the Snapshot.
func (s *Snapshot) Bytes() (serialized []byte) {
	return marshalutil.New().
		Write(s.LedgerSnapshot).
		Write(s.ManaSnapshot).
		Bytes()
}

func (s *Snapshot) String() (humanReadable string) {
	return stringify.Struct("Snapshot",
		stringify.StructField("LedgerSnapshot", s.LedgerSnapshot),
		stringify.StructField("ManaSnapshot", s.ManaSnapshot),
	)
}

func (s *Snapshot) takeManaSnapshot(accessManaByNode mana.NodeMap, accessManaTime time.Time) (snapshot *mana.Snapshot) {
	snapshot = &mana.Snapshot{
		ByNodeID: make(map[identity.ID]*mana.SnapshotNode),
	}

	for nodeID, accessManaValue := range accessManaByNode {
		snapshot.NodeSnapshot(nodeID).AccessMana = &mana.AccessManaSnapshot{
			Value:     accessManaValue,
			Timestamp: accessManaTime,
		}
	}

	_ = s.LedgerSnapshot.Outputs.ForEach(func(output utxo.Output) error {
		outputMetadata, exists := s.LedgerSnapshot.OutputsMetadata.Get(output.ID())
		if !exists {
			panic(fmt.Sprintf("output metadata with %s not found in snapshot", output.ID()))
		}

		s.updateConsensusManaDetails(snapshot.NodeSnapshot(outputMetadata.ConsensusManaPledgeID()), output.(devnetvm.Output), outputMetadata)

		return nil
	})

	return snapshot
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