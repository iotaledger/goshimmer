package ledger

import (
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// Snapshot represents a snapshot of the current ledger state.
type Snapshot struct {
	Header              *SnapshotHeader            `serix:"0"`
	OutputsWithMetadata []*OutputWithMetadata      `serix:"1,lengthPrefixType=uint32"`
	EpochDiffs          map[epoch.Index]*EpochDiff `serix:"2,lengthPrefixType=uint32"`
}

// SnapshotHeader represents the info of a snapshot.
type SnapshotHeader struct {
	OutputWithMetadataCount uint64          `serix:"0"`
	FullEpochIndex          epoch.Index     `serix:"1"`
	DiffEpochIndex          epoch.Index     `serix:"2"`
	LatestECRecord          *epoch.ECRecord `serix:"3"`
}

// NewSnapshot creates a new Snapshot from the given details.
func NewSnapshot(outputsWithMetadata []*OutputWithMetadata) (new *Snapshot) {
	return &Snapshot{
		Header:              &SnapshotHeader{OutputWithMetadataCount: uint64(len(outputsWithMetadata))},
		OutputsWithMetadata: outputsWithMetadata,
	}
}

// String returns a human-readable version of the Snapshot.
func (s *Snapshot) String() (humanReadable string) {
	structBuilder := stringify.StructBuilder("Snapshot")
	structBuilder.AddField(stringify.StructField("SnapshotHeader", s.Header))
	structBuilder.AddField(stringify.StructField("OutputsWithMetadata", s.OutputsWithMetadata))
	structBuilder.AddField(stringify.StructField("EpochDiffs", s.EpochDiffs))
	return structBuilder.String()
}

// String returns a human-readable version of the Snapshot.
func (h *SnapshotHeader) String() (humanReadable string) {
	return stringify.Struct("SnapshotHeader",
		stringify.StructField("OutputWithMetadataCount", h.OutputWithMetadataCount),
		stringify.StructField("FullEpochIndex", h.FullEpochIndex),
		stringify.StructField("DiffEpochIndex", h.DiffEpochIndex),
		stringify.StructField("LatestECRecord", h.LatestECRecord),
	)
}
