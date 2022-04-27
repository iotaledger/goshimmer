package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// Snapshot represents a snapshot of the current ledger state.
type Snapshot struct {
	Outputs         utxo.Outputs
	OutputsMetadata OutputsMetadata
}

// NewSnapshot creates a new Snapshot from the given details.
func NewSnapshot(outputs utxo.Outputs, outputsMetadata OutputsMetadata) (new *Snapshot) {
	return &Snapshot{
		Outputs:         outputs,
		OutputsMetadata: outputsMetadata,
	}
}

// FromMarshalUtil unserializes a Snapshot from the given .MarshalUtil.
func (s *Snapshot) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil, outputFactory utxo.OutputFactory) (err error) {
	if err = s.Outputs.FromMarshalUtil(marshalUtil, outputFactory); err != nil {
		return errors.Errorf("could not unmarshal outputs: %w", err)
	}
	if err = s.OutputsMetadata.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("could not unmarshal outputs metadata: %w", err)
	}

	return nil
}

// Bytes returns a serialized version of the Snapshot.
func (s *Snapshot) Bytes() (serialized []byte) {
	marshalUtil := marshalutil.New()
	marshalUtil.Write(s.Outputs)

	return marshalUtil.Bytes()
}
