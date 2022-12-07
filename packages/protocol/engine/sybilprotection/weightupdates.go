package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type WeightUpdatesBatch struct {
	targetEpoch epoch.Index
	diffs       map[identity.ID]int64
	totalDiff   int64
}

func NewWeightUpdates(targetEpoch epoch.Index) (newWeightUpdatesBatch *WeightUpdatesBatch) {
	return &WeightUpdatesBatch{
		targetEpoch: targetEpoch,
		diffs:       make(map[identity.ID]int64),
	}
}

func (w *WeightUpdatesBatch) ApplyDiff(index epoch.Index, id identity.ID, diff int64) {
	if w.diffs[id] += diff; w.diffs[id] == 0 {
		delete(w.diffs, id)
	}

	w.totalDiff += diff
}

func (w *WeightUpdatesBatch) TargetEpoch() (targetEpoch epoch.Index) {
	return w.targetEpoch
}

func (w *WeightUpdatesBatch) ForEach(consumer func(id identity.ID, diff int64)) {
	for id, diff := range w.diffs {
		consumer(id, diff)
	}
}

func (w *WeightUpdatesBatch) TotalDiff() (totalDiff int64) {
	return w.totalDiff
}
