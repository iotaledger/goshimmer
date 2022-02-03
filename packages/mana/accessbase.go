package mana

import (
	"math"
	"time"
)

// AccessBaseMana holds information about the access base mana values of a single node.
type AccessBaseMana struct {
	BaseMana2          float64
	EffectiveBaseMana2 float64
	LastUpdated        time.Time
}

func (a *AccessBaseMana) update(t time.Time) error {
	if t.Before(a.LastUpdated) || t == a.LastUpdated {
		// trying to do a time wise update to the past, that is not allowed
		return ErrAlreadyUpdated
	}
	n := t.Sub(a.LastUpdated)
	a.updateBM2(n)
	a.updateEBM2(n)
	a.LastUpdated = t
	return nil
}

func (a *AccessBaseMana) updateBM2(n time.Duration) {
	// zero value doesn't need to be updated
	if a.BaseMana2 == 0 {
		return
	}
	// close to zero value is considered zero, stop future updates
	if a.BaseMana2 < DeltaStopUpdate {
		a.BaseMana2 = 0
		return
	}
	a.BaseMana2 *= math.Pow(math.E, -Decay*n.Seconds())
}

func (a *AccessBaseMana) updateEBM2(n time.Duration) {
	// zero value doesn't need to be updated
	if a.BaseMana2 == 0 && a.EffectiveBaseMana2 == 0 {
		return
	}
	// close to zero value is considered zero, stop future updates
	if a.BaseMana2 == 0 && a.EffectiveBaseMana2 < DeltaStopUpdate {
		a.EffectiveBaseMana2 = 0
		return
	}
	if emaCoeff2 != Decay {
		a.EffectiveBaseMana2 = math.Pow(math.E, -emaCoeff2*n.Seconds())*a.EffectiveBaseMana2 +
			(1-math.Pow(math.E, -(emaCoeff2-Decay)*n.Seconds()))/
				(emaCoeff2-Decay)*emaCoeff2*a.BaseMana2
	} else {
		a.EffectiveBaseMana2 = math.Pow(math.E, -Decay*n.Seconds())*a.EffectiveBaseMana2 +
			Decay*n.Seconds()*a.BaseMana2
	}
}

func (a *AccessBaseMana) revoke(float64) error {
	panic("access mana cannot be revoked")
}

func (a *AccessBaseMana) pledge(tx *TxInfo) (pledged float64) {
	t := tx.TimeStamp

	if t.After(a.LastUpdated) {
		// regular update
		n := t.Sub(a.LastUpdated)
		// first, update BM2 and EBM2 until `t`
		a.updateBM2(n)
		a.updateEBM2(n)
		a.LastUpdated = t
		// pending mana awarded, need to see how long funds sat
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -Decay*(t.Sub(input.TimeStamp).Seconds())))
			a.BaseMana2 += bm2Add
			pledged += bm2Add
		}
	} else {
		// past update
		n := a.LastUpdated.Sub(t)
		// update  BM2 at `t`
		oldMana2 := a.BaseMana2
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -Decay*(t.Sub(input.TimeStamp).Seconds()))) *
				math.Pow(math.E, -Decay*n.Seconds())
			a.BaseMana2 += bm2Add
			pledged += bm2Add
		}
		// update EBM2 to `bm.LastUpdated`
		if emaCoeff2 != Decay {
			a.EffectiveBaseMana2 += (a.BaseMana2 - oldMana2) * emaCoeff2 * (math.Pow(math.E, -Decay*n.Seconds()) -
				math.Pow(math.E, -emaCoeff2*n.Seconds())) / (emaCoeff2 - Decay) / math.Pow(math.E, -Decay*n.Seconds())
		} else {
			a.EffectiveBaseMana2 += (a.BaseMana2 - oldMana2) * Decay * n.Seconds()
		}
	}
	return
}

// BaseValue returns the base mana value (BM2).
func (a *AccessBaseMana) BaseValue() float64 {
	return a.BaseMana2
}

// EffectiveValue returns the effective base mana value (EBM2).
func (a *AccessBaseMana) EffectiveValue() float64 {
	return a.EffectiveBaseMana2
}

// LastUpdate returns the last update time.
func (a *AccessBaseMana) LastUpdate() time.Time {
	return a.LastUpdated
}

var _ BaseMana = &AccessBaseMana{}
