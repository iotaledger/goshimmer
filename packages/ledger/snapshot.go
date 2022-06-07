package ledger

import (
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// Snapshot represents a snapshot of the current ledger state.
type Snapshot struct {
	Outputs         *utxo.Outputs     `serix:"0"`
	OutputsMetadata *OutputsMetadata  `serix:"1"`
	FullEpochIndex  epoch.EI          `serix:"2"`
	DiffEpochIndex  epoch.EI          `serix:"3"`
	EpochDiffs      *epoch.EpochDiffs `serix:"4"`

	// EC of DiffEpochIndex
	EC epoch.EC `serix:"5"`
}

// NewSnapshot creates a new Snapshot from the given details.
func NewSnapshot(outputs *utxo.Outputs, outputsMetadata *OutputsMetadata) (new *Snapshot) {
	return &Snapshot{
		Outputs:         outputs,
		OutputsMetadata: outputsMetadata,
	}
}

// String returns a human-readable version of the Snapshot.
func (s *Snapshot) String() (humanReadable string) {
	return stringify.Struct("Snapshot",
		stringify.StructField("Outputs", s.Outputs),
		stringify.StructField("OutputsMetadata", s.OutputsMetadata),
	)
}
