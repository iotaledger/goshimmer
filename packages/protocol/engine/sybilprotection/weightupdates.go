package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type WeightUpdates struct {
	sourceEpoch epoch.Index
	targetEpoch epoch.Index
	diffs       map[identity.ID]int64
	totalDiff int64
}

func NewWeightUpdates(sourceEpoch, targetEpoch epoch.Index) (newWeightUpdates *WeightUpdates) {
	return &WeightUpdates{
		sourceEpoch: sourceEpoch,
		targetEpoch: targetEpoch,
		diffs:       make(map[identity.ID]int64),
	}
}

func (w *WeightUpdates) ApplyDiff(id identity.ID, diff int64) {
	if w.diffs[id] += diff; w.diffs[id] == 0 {
		delete(w.diffs, id)
	}

	w.totalDiff += diff
}

func (w *WeightUpdates) SourceEpoch() (targetEpoch epoch.Index) {
	return w.sourceEpoch
}

func (w *WeightUpdates) TargetEpoch() (targetEpoch epoch.Index) {
	return w.targetEpoch
}

func (w *WeightUpdates) Direction() int64 {
	return int64(lo.Compare(w.TargetEpoch(), w.sourceEpoch))
}

func (w *WeightUpdates) ForEach(consumer func(id identity.ID, diff int64)) {
	for id, diff := range w.diffs {
		consumer(id, diff)
	}
}

func (w *WeightUpdates) TotalDiff() (totalDiff int64) {
	return w.totalDiff
}
