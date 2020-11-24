package mana

import (
	"time"

	"golang.org/x/xerrors"
)

// WeightedBaseMana holds information about the combined mana1+mana2 base values of a node.
type WeightedBaseMana struct {
	mana1  *ConsensusBaseMana
	mana2  *AccessBaseMana
	weight float64
}

// NewWeightedMana creates a new *WeightedMana object.
func NewWeightedMana(weight float64) *WeightedBaseMana {
	result := &WeightedBaseMana{
		mana1: &ConsensusBaseMana{},
		mana2: &AccessBaseMana{},
	}
	if err := result.SetWeight(weight); err != nil {
		panic(err)
	}
	return result
}

func (w *WeightedBaseMana) update(t time.Time) error {
	err := w.mana1.update(t)
	if err != nil {
		return err
	}
	return w.mana2.update(t)
}

func (w *WeightedBaseMana) revoke(value float64, t time.Time) error {
	// synchronize update times, since mana1 could potentially update
	_ = w.update(t)
	return w.mana1.revoke(value, t)
}

func (w *WeightedBaseMana) pledge(tx *TxInfo) float64 {
	mana1Pledged := w.mana1.pledge(tx)
	return w.mana2.pledge(tx)*(1-w.weight) + mana1Pledged*w.weight
}

// BaseValue returns the base mana value, that is the weighted composition of BM1 and BM2.
// result = BM1 * weight + BM2 * (1 - weight)
func (w *WeightedBaseMana) BaseValue() float64 {
	return w.mana1.BaseValue()*w.weight + w.mana2.BaseValue()*(1-w.weight)
}

// EffectiveValue returns the effective base mana value, that is the weighted composition of EBM1 and EBM2.
// result = EBM1 * weight + EBM2 * (1 - weight)
func (w *WeightedBaseMana) EffectiveValue() float64 {
	return w.mana1.EffectiveValue()*w.weight + w.mana2.EffectiveValue()*(1-w.weight)
}

// LastUpdate returns the last update time.
func (w *WeightedBaseMana) LastUpdate() time.Time {
	// besides revoking, update times are the same for both components.
	if w.mana1.LastUpdate().Before(w.mana2.LastUpdate()) {
		return w.mana1.LastUpdate()
	}
	return w.mana2.LastUpdate()
}

// SetWeight sets the weight, that has to be within [0,1], otherwise an error is returned.
func (w *WeightedBaseMana) SetWeight(weight float64) error {
	if weight < 0.0 || weight > 1.0 {
		return xerrors.Errorf("error while setting weight to %f: %w", weight, ErrInvalidWeightParameter)
	}
	w.weight = weight
	return nil
}

var _ BaseMana = &WeightedBaseMana{}
