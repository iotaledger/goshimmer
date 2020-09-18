package mana

import (
	"errors"
	"math"
	"time"
)

var (
	// ErrAlreadyUpdated is returned if mana is tried to be updated at a later time.
	ErrAlreadyUpdated = errors.New("already updated to a later timestamp")
	// ErrBaseManaNegative is returned if base mana will become negative.
	ErrBaseManaNegative = errors.New("base mana should never be negative")
)

// BaseMana holds information about the base mana values of a single node.
type BaseMana struct {
	BaseMana1          float64
	EffectiveBaseMana1 float64
	BaseMana2          float64
	EffectiveBaseMana2 float64
	LastUpdated        time.Time
}

func (bm *BaseMana) update(t time.Time) error {
	if t.Before(bm.LastUpdated) || t == bm.LastUpdated {
		// trying to do a time wise update to the past, that is not allowed
		return ErrAlreadyUpdated
	}
	n := t.Sub(bm.LastUpdated)
	bm.updateEBM1(n)
	bm.updateBM2(n)
	bm.updateEBM2(n)

	bm.LastUpdated = t
	return nil
}

func (bm *BaseMana) updateEBM1(n time.Duration) {
	bm.EffectiveBaseMana1 = math.Pow(math.E, -emaCoeff1*n.Seconds())*bm.EffectiveBaseMana1 +
		(1-math.Pow(math.E, -emaCoeff1*n.Seconds()))*bm.BaseMana1
}

func (bm *BaseMana) updateBM2(n time.Duration) {
	bm.BaseMana2 = bm.BaseMana2 * math.Pow(math.E, -decay*n.Seconds())
}

func (bm *BaseMana) updateEBM2(n time.Duration) {
	if emaCoeff2 != decay {
		bm.EffectiveBaseMana2 = math.Pow(math.E, -emaCoeff2*n.Seconds())*bm.EffectiveBaseMana2 +
			(math.Pow(math.E, -decay*n.Seconds())-math.Pow(math.E, -emaCoeff2*n.Seconds()))/
				(emaCoeff2-decay)*emaCoeff2/math.Pow(math.E, -decay*n.Seconds())*bm.BaseMana2
	} else {
		bm.EffectiveBaseMana2 = math.Pow(math.E, -decay*n.Seconds())*bm.EffectiveBaseMana2 +
			decay*n.Seconds()*bm.BaseMana2
	}
}

func (bm *BaseMana) revokeBaseMana1(amount float64, t time.Time) error {
	if bm.BaseMana1-amount < 0.0 {
		return ErrBaseManaNegative
	}
	if t.After(bm.LastUpdated) {
		// regular update
		n := t.Sub(bm.LastUpdated)
		// first, update EBM1, BM2 and EBM2 until `t`
		bm.updateEBM1(n)
		bm.updateBM2(n)
		bm.updateEBM2(n)

		bm.LastUpdated = t
		// revoke BM1 at `t`
		bm.BaseMana1 -= amount
	} else {
		// update in past
		n := bm.LastUpdated.Sub(t)
		// revoke BM1 at `t`
		bm.BaseMana1 -= amount
		// update EBM1 to `bm.LastUpdated`
		EBM1Compensation := amount * (1 - math.Pow(math.E, -emaCoeff1*n.Seconds()))
		if bm.EffectiveBaseMana1-EBM1Compensation < 0 {
			panic("Effective Base Mana 1 Should never be negative")
		} else {
			bm.EffectiveBaseMana1 -= EBM1Compensation
		}
	}
	return nil
}

func (bm *BaseMana) pledgeAndUpdate(tx *TxInfo) (bm1Pledged float64, bm2Pledged float64) {
	t := tx.TimeStamp
	bm1Pledged = tx.sumInputs()

	if t.After(bm.LastUpdated) {
		// regular update
		n := t.Sub(bm.LastUpdated)
		// first, update EBM1, BM2 and EBM2 until `t`
		bm.updateEBM1(n)
		bm.updateBM2(n)
		bm.updateEBM2(n)
		bm.LastUpdated = t
		bm.BaseMana1 += bm1Pledged
		// pending mana awarded, need to see how long funds sat
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -decay*(t.Sub(input.TimeStamp).Seconds())))
			bm.BaseMana2 += bm2Add
			bm2Pledged += bm2Add
		}
	} else {
		// past update
		n := bm.LastUpdated.Sub(t)
		// update BM1 and BM2 at `t`
		bm.BaseMana1 += bm1Pledged
		oldMana2 := bm.BaseMana2
		for _, input := range tx.InputInfos {
			bm2Add := input.Amount * (1 - math.Pow(math.E, -decay*(t.Sub(input.TimeStamp).Seconds()))) *
				math.Pow(math.E, -decay*n.Seconds())
			bm.BaseMana2 += bm2Add
			bm2Pledged += bm2Add
		}
		// update EBM1 and EBM2 to `bm.LastUpdated`
		bm.EffectiveBaseMana1 += bm1Pledged * (1 - math.Pow(math.E, -emaCoeff1*n.Seconds()))
		if emaCoeff2 != decay {
			bm.EffectiveBaseMana2 += (bm.BaseMana2 - oldMana2) * emaCoeff2 * (math.Pow(math.E, -decay*n.Seconds()) -
				math.Pow(math.E, -emaCoeff2*n.Seconds())) / (emaCoeff2 - decay) / math.Pow(math.E, -decay*n.Seconds())
		} else {
			bm.EffectiveBaseMana2 += (bm.BaseMana2 - oldMana2) * decay * n.Seconds()
		}
	}
	return bm1Pledged, bm2Pledged
}
