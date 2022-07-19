package ledger

import (
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// Snapshot represents a snapshot of the current ledger state.
type Snapshot struct {
	OutputsWithMetadata []*OutputWithMetadata      `serix:"0,lengthPrefixType=uint32"`
	FullEpochIndex      epoch.Index                `serix:"1"`
	DiffEpochIndex      epoch.Index                `serix:"2"`
	EpochDiffs          map[epoch.Index]*EpochDiff `serix:"3,lengthPrefixType=uint32"`
	EpochActiveNodes    epoch.NodesActivityLog     `serix:"4,lengthPrefixType=uint32"`
	LatestECRecord      *epoch.ECRecord            `serix:"5"`
}

// NewSnapshot creates a new Snapshot from the given details.
func NewSnapshot(outputsWithMetadata []*OutputWithMetadata) (new *Snapshot) {
	return &Snapshot{
		OutputsWithMetadata: outputsWithMetadata,
	}
}

// String returns a human-readable version of the Snapshot.
func (s *Snapshot) String() (humanReadable string) {
	return stringify.Struct("Snapshot",
		stringify.StructField("OutputsWithMetadata", s.OutputsWithMetadata),
		stringify.StructField("FullEpochIndex", s.FullEpochIndex),
		stringify.StructField("DiffEpochIndex", s.DiffEpochIndex),
		stringify.StructField("EpochDiffs", s.EpochDiffs),
		stringify.StructField("EpochActiveNodes", s.EpochActiveNodes),
		stringify.StructField("LatestECRecord", s.LatestECRecord),
	)
}
