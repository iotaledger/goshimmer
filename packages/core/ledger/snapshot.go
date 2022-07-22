package ledger

import (
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// Snapshot represents a snapshot of the current ledger state.
type Snapshot struct {
	OutputWithMetadataCount uint64                     `serix:"0"`
	FullEpochIndex          epoch.Index                `serix:"1"`
	DiffEpochIndex          epoch.Index                `serix:"2"`
	LatestECRecord          *epoch.ECRecord            `serix:"3"`
	EpochDiffs              map[epoch.Index]*EpochDiff `serix:"4,lengthPrefixType=uint32"`
	OutputsWithMetadata     []*OutputWithMetadata      `serix:"5,lengthPrefixType=uint32"`
}

// NewSnapshot creates a new Snapshot from the given details.
func NewSnapshot(outputsWithMetadata []*OutputWithMetadata) (new *Snapshot) {
	return &Snapshot{
		OutputWithMetadataCount: uint64(len(outputsWithMetadata)),
		OutputsWithMetadata:     outputsWithMetadata,
	}
}

// String returns a human-readable version of the Snapshot.
func (s *Snapshot) String() (humanReadable string) {
	return stringify.Struct("Snapshot",
		stringify.StructField("OutputWithMetadataCount", s.OutputWithMetadataCount),
		stringify.StructField("OutputsWithMetadata", s.OutputsWithMetadata),
		stringify.StructField("FullEpochIndex", s.FullEpochIndex),
		stringify.StructField("DiffEpochIndex", s.DiffEpochIndex),
		stringify.StructField("EpochDiffs", s.EpochDiffs),
		stringify.StructField("LatestECRecord", s.LatestECRecord),
	)
}
