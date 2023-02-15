package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// WeightsBatch is a batch of weight diffs that can be applied to a Weights instance.
type WeightsBatch struct {
	targetEpoch epoch.Index
	diffs       map[identity.ID]int64
	totalDiff   int64
}

// NewWeightsBatch creates a new WeightsBatch instance.
func NewWeightsBatch(targetEpoch epoch.Index) (newWeightDiffs *WeightsBatch) {
	return &WeightsBatch{
		targetEpoch: targetEpoch,
		diffs:       make(map[identity.ID]int64),
	}
}

// Update updates the weight diff of the given identity.
func (w *WeightsBatch) Update(id identity.ID, diff int64) {
	if w.diffs[id] += diff; w.diffs[id] == 0 {
		delete(w.diffs, id)
	}

	w.totalDiff += diff
}

func (w *WeightsBatch) Get(id identity.ID) (diff int64) {
	return w.diffs[id]
}

// TargetEpoch returns the epoch that the batch is targeting.
func (w *WeightsBatch) TargetEpoch() (targetEpoch epoch.Index) {
	return w.targetEpoch
}

// ForEach iterates over all weight diffs in the batch.
func (w *WeightsBatch) ForEach(consumer func(id identity.ID, diff int64)) {
	for id, diff := range w.diffs {
		consumer(id, diff)
	}
}

// TotalDiff returns the total weight diff of the batch.
func (w *WeightsBatch) TotalDiff() (totalDiff int64) {
	return w.totalDiff
}
