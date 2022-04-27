package snapshot

import (
	"os"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/mana"
)

type Snapshot struct {
	LedgerSnapshot *ledger.Snapshot
	ManaSnapshot   *mana.Snapshot
}

func (s *Snapshot) FromNode(ledger *ledger.Ledger) {
	s.LedgerSnapshot = ledger.TakeSnapshot()
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
	if err = s.LedgerSnapshot.FromMarshalUtil(marshalUtil, devnetvm.OutputFactory); err != nil {
		return errors.Errorf("failed to read LedgerSnapshot: %w", err)
	}
	if err = s.ManaSnapshot.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to read ManaSnapshot: %w", err)
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
