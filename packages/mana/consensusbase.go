package mana

import (
	"time"
)

// ConsensusBaseMana holds information about the consensus base mana values of a single node.
type ConsensusBaseMana struct {
	BaseMana1 float64
}

func (c *ConsensusBaseMana) update(now time.Time) error {
	panic("not implemented")
}

func (c *ConsensusBaseMana) revoke(amount float64) error {
	//if c.BaseMana1-amount < 0.0 {
	//	return ErrBaseManaNegative
	//}
	c.BaseMana1 -= amount
	return nil
}

func (c *ConsensusBaseMana) pledge(tx *TxInfo) (pledged float64) {
	pledged = tx.sumInputs()
	c.BaseMana1 += pledged
	return pledged
}

// BaseValue returns the base mana value (BM1).
func (c *ConsensusBaseMana) BaseValue() float64 {
	return c.BaseMana1
}

// EffectiveValue returns the effective base mana value (EBM1).
func (c *ConsensusBaseMana) EffectiveValue() float64 {
	panic("not implemented")
}

// LastUpdate returns the last update time.
func (c *ConsensusBaseMana) LastUpdate() time.Time {
	panic("not implemented")
}

var _ BaseMana = &ConsensusBaseMana{}
