package mana

import (
	"math"
	"time"

	"github.com/iotaledger/hive.go/generics/model"
)

// AccessBaseMana holds information about the access base mana values of a single node.
type AccessBaseMana struct {
	model.Model[accessBaseManaModel]
}

type accessBaseManaModel struct {
	BaseMana2          float64
	EffectiveBaseMana2 float64
	LastUpdated        time.Time
}

// NewAccessBaseMana returns new base access mana vector.
func NewAccessBaseMana(baseMana, effectiveBaseMana float64, lastUpdated time.Time) *AccessBaseMana {
	return &AccessBaseMana{
		model.New[accessBaseManaModel](accessBaseManaModel{
			BaseMana2:          baseMana,
			EffectiveBaseMana2: effectiveBaseMana,
			LastUpdated:        lastUpdated,
		}),
	}
}

// EffectiveValue returns effective base mana value.
func (a *AccessBaseMana) EffectiveValue() float64 {
	a.RLock()
	defer a.RUnlock()
	return a.M.EffectiveBaseMana2
}

// BaseValue returns base mana value.
func (a *AccessBaseMana) BaseValue() float64 {
	a.RLock()
	defer a.RUnlock()
	return a.M.BaseMana2
}

// LastUpdate returns last update time.
func (a *AccessBaseMana) LastUpdate() time.Time {
	a.RLock()
	defer a.RUnlock()
	return a.M.LastUpdated
}

func (a *AccessBaseMana) update(t time.Time) error {
	if t.Before(a.LastUpdate()) || t == a.LastUpdate() {
		// trying to do a time wise update to the past, that is not allowed
		return ErrAlreadyUpdated
	}
	n := t.Sub(a.LastUpdate())
	a.updateBM2(n)
	a.updateEBM2(n)
	a.M.LastUpdated = t
	return nil
}

func (a *AccessBaseMana) updateBM2(n time.Duration) {
	// zero value doesn't need to be updated
	if a.BaseValue() == 0 {
		return
	}
	// close to zero value is considered zero, stop future updates
	if a.BaseValue() < DeltaStopUpdate {
		a.M.BaseMana2 = 0
		return
	}
	a.M.BaseMana2 *= math.Pow(math.E, -Decay*n.Seconds())
}

func (a *AccessBaseMana) updateEBM2(n time.Duration) {
	// zero value doesn't need to be updated
	if a.BaseValue() == 0 && a.EffectiveValue() == 0 {
		return
	}
	// close to zero value is considered zero, stop future updates
	if a.BaseValue() == 0 && a.EffectiveValue() < DeltaStopUpdate {
		a.M.EffectiveBaseMana2 = 0
		return
	}
	if emaCoeff2 != Decay {
		a.M.EffectiveBaseMana2 = math.Pow(math.E, -emaCoeff2*n.Seconds())*a.EffectiveValue() +
			(1-math.Pow(math.E, -(emaCoeff2-Decay)*n.Seconds()))/
				(emaCoeff2-Decay)*emaCoeff2*a.BaseValue()
	} else {
		a.M.EffectiveBaseMana2 = math.Pow(math.E, -Decay*n.Seconds())*a.EffectiveValue() +
			Decay*n.Seconds()*a.BaseValue()
	}
}

func (a *AccessBaseMana) revoke(float64) error {
	panic("access mana cannot be revoked")
}

func (a *AccessBaseMana) pledge(tx *TxInfo) (pledged float64) {
	t := tx.TimeStamp

	if t.After(a.LastUpdate()) {
		// regular update
		n := t.Sub(a.LastUpdate())
		// first, update BM2 and EBM2 until `t`
		a.updateBM2(n)
		a.updateEBM2(n)
		a.M.LastUpdated = t
		// pending mana awarded, need to see how long funds sat
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -Decay*(t.Sub(input.TimeStamp).Seconds())))
			a.M.BaseMana2 += bm2Add
			pledged += bm2Add
		}
	} else {
		// past update
		n := a.LastUpdate().Sub(t)
		// update  BM2 at `t`
		oldMana2 := a.BaseValue()
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -Decay*(t.Sub(input.TimeStamp).Seconds()))) *
				math.Pow(math.E, -Decay*n.Seconds())
			a.M.BaseMana2 += bm2Add
			pledged += bm2Add
		}
		// update EBM2 to `bm.LastUpdate`
		if emaCoeff2 != Decay {
			a.M.EffectiveBaseMana2 += (a.BaseValue() - oldMana2) * emaCoeff2 * (math.Pow(math.E, -Decay*n.Seconds()) -
				math.Pow(math.E, -emaCoeff2*n.Seconds())) / (emaCoeff2 - Decay) / math.Pow(math.E, -Decay*n.Seconds())
		} else {
			a.M.EffectiveBaseMana2 += (a.BaseValue() - oldMana2) * Decay * n.Seconds()
		}
	}
	return
}

var _ BaseMana = &AccessBaseMana{}
