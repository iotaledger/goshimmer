package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type WeightsBatch struct {
	targetEpoch epoch.Index
	diffs       map[identity.ID]int64
	totalDiff   int64
}

func NewWeightsBatch(targetEpoch epoch.Index) (newWeightDiffs *WeightsBatch) {
	return &WeightsBatch{
		targetEpoch: targetEpoch,
		diffs:       make(map[identity.ID]int64),
	}
}

func (w *WeightsBatch) Update(id identity.ID, diff int64) {
	if w.diffs[id] += diff; w.diffs[id] == 0 {
		delete(w.diffs, id)
	}

	w.totalDiff += diff
}

func (w *WeightsBatch) TargetEpoch() (targetEpoch epoch.Index) {
	return w.targetEpoch
}

func (w *WeightsBatch) ForEach(consumer func(id identity.ID, diff int64)) {
	for id, diff := range w.diffs {
		consumer(id, diff)
	}
}

func (w *WeightsBatch) TotalDiff() (totalDiff int64) {
	return w.totalDiff
}
