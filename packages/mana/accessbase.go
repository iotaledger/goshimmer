package mana

import (
	"math"
	"time"

	"github.com/iotaledger/hive.go/generics/model"
)

// AccessBaseMana holds information about the access base mana values of a single node.
type AccessBaseMana struct {
	model.Mutable[AccessBaseMana, *AccessBaseMana, accessBaseManaModel] `serix:"0"`
}

type accessBaseManaModel struct {
	BaseValue      float64   `serix:"0"`
	EffectiveValue float64   `serix:"1"`
	LastUpdate     time.Time `serix:"2"`
}

// NewAccessBaseMana returns new base access mana vector.
func NewAccessBaseMana(baseMana, effectiveBaseMana float64, lastUpdated time.Time) (newAccessBaseMana *AccessBaseMana) {
	return model.NewMutable[AccessBaseMana](&accessBaseManaModel{
		BaseValue:      baseMana,
		EffectiveValue: effectiveBaseMana,
		LastUpdate:     lastUpdated,
	})
}

// EffectiveValue returns effective base mana value.
func (a *AccessBaseMana) EffectiveValue() float64 {
	a.RLock()
	defer a.RUnlock()

	return a.M.EffectiveValue
}

// BaseValue returns base mana value.
func (a *AccessBaseMana) BaseValue() float64 {
	a.RLock()
	defer a.RUnlock()

	return a.M.BaseValue
}

// LastUpdate returns last update time.
func (a *AccessBaseMana) LastUpdate() time.Time {
	a.RLock()
	defer a.RUnlock()

	return a.M.LastUpdate
}

func (a *AccessBaseMana) update(t time.Time) error {
	if t.Before(a.LastUpdate()) || t == a.LastUpdate() {
		// trying to do a time wise update to the past, that is not allowed
		return ErrAlreadyUpdated
	}
	n := t.Sub(a.LastUpdate())
	a.updateBM2(n)
	a.updateEBM2(n)
	a.M.LastUpdate = t
	return nil
}

func (a *AccessBaseMana) updateBM2(n time.Duration) {
	// zero value doesn't need to be updated
	if a.BaseValue() == 0 {
		return
	}
	// close to zero value is considered zero, stop future updates
	if a.BaseValue() < DeltaStopUpdate {
		a.M.BaseValue = 0
		return
	}
	a.M.BaseValue *= math.Pow(math.E, -Decay*n.Seconds())
}

func (a *AccessBaseMana) updateEBM2(n time.Duration) {
	// zero value doesn't need to be updated
	if a.BaseValue() == 0 && a.EffectiveValue() == 0 {
		return
	}
	// close to zero value is considered zero, stop future updates
	if a.BaseValue() == 0 && a.EffectiveValue() < DeltaStopUpdate {
		a.M.EffectiveValue = 0
		return
	}
	if emaCoeff2 != Decay {
		a.M.EffectiveValue = math.Pow(math.E, -emaCoeff2*n.Seconds())*a.EffectiveValue() +
			(1-math.Pow(math.E, -(emaCoeff2-Decay)*n.Seconds()))/
				(emaCoeff2-Decay)*emaCoeff2*a.BaseValue()
	} else {
		a.M.EffectiveValue = math.Pow(math.E, -Decay*n.Seconds())*a.EffectiveValue() +
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
		a.M.LastUpdate = t
		// pending mana awarded, need to see how long funds sat
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -Decay*(t.Sub(input.TimeStamp).Seconds())))
			a.M.BaseValue += bm2Add
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
			a.M.BaseValue += bm2Add
			pledged += bm2Add
		}
		// update EBM2 to `bm.LastUpdate`
		if emaCoeff2 != Decay {
			a.M.EffectiveValue += (a.BaseValue() - oldMana2) * emaCoeff2 * (math.Pow(math.E, -Decay*n.Seconds()) -
				math.Pow(math.E, -emaCoeff2*n.Seconds())) / (emaCoeff2 - Decay) / math.Pow(math.E, -Decay*n.Seconds())
		} else {
			a.M.EffectiveValue += (a.BaseValue() - oldMana2) * Decay * n.Seconds()
		}
	}
	return
}

var _ BaseMana = &AccessBaseMana{}
