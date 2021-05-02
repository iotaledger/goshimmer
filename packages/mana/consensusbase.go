package mana

import (
	"math"
	"time"
)

// ConsensusBaseMana holds information about the consensus base mana values of a single node.
type ConsensusBaseMana struct {
	BaseMana1          float64
	EffectiveBaseMana1 float64
	LastUpdated        time.Time
}

func (c *ConsensusBaseMana) update(t time.Time) error {
	if t.Before(c.LastUpdated) || t == c.LastUpdated {
		// trying to do a time wise update to the past, that is not allowed
		return ErrAlreadyUpdated
	}
	n := t.Sub(c.LastUpdated)
	c.updateEBM1(n)
	c.LastUpdated = t
	return nil
}

func (c *ConsensusBaseMana) updateEBM1(n time.Duration) {
	// base and effective value is the same, no need to update.
	if c.BaseMana1 == c.EffectiveBaseMana1 {
		return
	}
	// normal update
	c.EffectiveBaseMana1 = math.Pow(math.E, -emaCoeff1*n.Seconds())*c.EffectiveBaseMana1 +
		(1-math.Pow(math.E, -emaCoeff1*n.Seconds()))*c.BaseMana1
	// they are not the same, but close. Stop the future updates.
	if math.Abs(c.BaseMana1-c.EffectiveBaseMana1) < DeltaStopUpdate {
		c.EffectiveBaseMana1 = c.BaseMana1
	}
}

func (c *ConsensusBaseMana) revoke(amount float64, t time.Time) error {
	if c.BaseMana1-amount < 0.0 {
		return ErrBaseManaNegative
	}
	if t.After(c.LastUpdated) {
		// regular update
		n := t.Sub(c.LastUpdated)
		// first, update EBM1`t`
		c.updateEBM1(n)
		c.LastUpdated = t
		// revoke BM1 at `t`
		c.BaseMana1 -= amount
	} else {
		// update in past
		n := c.LastUpdated.Sub(t)
		// revoke BM1 at `t`
		c.BaseMana1 -= amount
		// update EBM1 to `bm.LastUpdated`
		EBM1Compensation := amount * (1 - math.Pow(math.E, -emaCoeff1*n.Seconds()))
		if c.EffectiveBaseMana1-EBM1Compensation < 0.0 {
			return ErrEffBaseManaNegative
		}
		c.EffectiveBaseMana1 -= EBM1Compensation
	}
	return nil
}

func (c *ConsensusBaseMana) pledge(tx *TxInfo) (pledged float64) {
	t := tx.TimeStamp
	pledged = tx.sumInputs()

	if t.After(c.LastUpdated) {
		// regular update
		n := t.Sub(c.LastUpdated)
		// first, update EBM1`t`
		c.updateEBM1(n)
		c.LastUpdated = t
		c.BaseMana1 += pledged
	} else {
		// past update
		n := c.LastUpdated.Sub(t)
		// update BM1 at `t`
		c.BaseMana1 += pledged
		// update EBM1 to `bm.LastUpdated`
		c.EffectiveBaseMana1 += pledged * (1 - math.Pow(math.E, -emaCoeff1*n.Seconds()))
	}
	return pledged
}

// BaseValue returns the base mana value (BM1).
func (c *ConsensusBaseMana) BaseValue() float64 {
	return c.BaseMana1
}

// EffectiveValue returns the effective base mana value (EBM1).
func (c *ConsensusBaseMana) EffectiveValue() float64 {
	return c.EffectiveBaseMana1
}

// LastUpdate returns the last update time.
func (c *ConsensusBaseMana) LastUpdate() time.Time {
	return c.LastUpdated
}

var _ BaseMana = &ConsensusBaseMana{}
