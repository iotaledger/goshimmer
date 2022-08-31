package ledger

import (
	"github.com/iotaledger/hive.go/core/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// Snapshot represents a snapshot of the current ledger state.
type Snapshot struct {
	Header              *SnapshotHeader             `serix:"0"`
	OutputsWithMetadata []*OutputWithMetadata       `serix:"1,lengthPrefixType=uint32"`
	EpochDiffs          map[epoch.Index]*EpochDiff  `serix:"2,lengthPrefixType=uint32"`
	EpochActiveNodes    epoch.SnapshotEpochActivity `serix:"3,lengthPrefixType=uint32"`
}

// SnapshotHeader represents the info of a snapshot.
type SnapshotHeader struct {
	OutputWithMetadataCount uint64          `serix:"0"`
	FullEpochIndex          epoch.Index     `serix:"1"`
	DiffEpochIndex          epoch.Index     `serix:"2"`
	LatestECRecord          *epoch.ECRecord `serix:"3"`
}

// NewSnapshot creates a new Snapshot from the given details.
func NewSnapshot(outputsWithMetadata []*OutputWithMetadata, activeNodes epoch.SnapshotEpochActivity) (new *Snapshot) {
	return &Snapshot{
		Header:              &SnapshotHeader{OutputWithMetadataCount: uint64(len(outputsWithMetadata))},
		OutputsWithMetadata: outputsWithMetadata,
		EpochActiveNodes:    activeNodes,
	}
}

// String returns a human-readable version of the Snapshot.
func (s *Snapshot) String() (humanReadable string) {
	structBuilder := stringify.NewStructBuilder("Snapshot")
	structBuilder.AddField(stringify.NewStructField("SnapshotHeader", s.Header))
	structBuilder.AddField(stringify.NewStructField("OutputsWithMetadata", s.OutputsWithMetadata))
	structBuilder.AddField(stringify.NewStructField("EpochDiffs", s.EpochDiffs))
	structBuilder.AddField(stringify.NewStructField("EpochActiveNodes", s.EpochActiveNodes))
	return structBuilder.String()
}

// String returns a human-readable version of the SnapshotHeader.
func (h *SnapshotHeader) String() (humanReadable string) {
	return stringify.Struct("SnapshotHeader",
		stringify.NewStructField("OutputWithMetadataCount", h.OutputWithMetadataCount),
		stringify.NewStructField("FullEpochIndex", h.FullEpochIndex),
		stringify.NewStructField("DiffEpochIndex", h.DiffEpochIndex),
		stringify.NewStructField("LatestECRecord", h.LatestECRecord),
	)
}
