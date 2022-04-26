package snapshot

import (
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
)

type Snapshot struct {
	LedgerSnapshot []utxo.Output
	ManaSnapshot   *mana.Snapshot
}

func (s *Snapshot) FromNode(ledger *ledger.Ledger) {
	s.LedgerSnapshot = ledger.Utils.UnspentOutputs()
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
	outputCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("failed to read output count: %w", err)
	}
	s.LedgerSnapshot = make([]utxo.Output, outputCount)
	for i := uint64(0); i < outputCount; i++ {
		if s.LedgerSnapshot[i], err = devnetvm.OutputFromMarshalUtil(marshalUtil); err != nil {
			return errors.Errorf("failed to read Output %d: %w", i, err)
		}
	}

	if err = s.ManaSnapshot.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to unmarshal ManaSnapshot: %w", err)
	}

	return nil
}

func (s *Snapshot) Bytes() (serialized []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(len(s.LedgerSnapshot)))
	for _, output := range s.LedgerSnapshot {
		marshalUtil.Write(output)
	}

	marshalUtil.Write(s.ManaSnapshot)

	return marshalUtil.Bytes()
}
