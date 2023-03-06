package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
)

// WeightsBatch is a batch of weight diffs that can be applied to a Weights instance.
type WeightsBatch struct {
	targetSlot slot.Index
	diffs      map[identity.ID]int64
	totalDiff  int64
}

// NewWeightsBatch creates a new WeightsBatch instance.
func NewWeightsBatch(targetSlot slot.Index) (newWeightDiffs *WeightsBatch) {
	return &WeightsBatch{
		targetSlot: targetSlot,
		diffs:      make(map[identity.ID]int64),
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

// TargetSlot returns the slot that the batch is targeting.
func (w *WeightsBatch) TargetSlot() (targetSlot slot.Index) {
	return w.targetSlot
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
