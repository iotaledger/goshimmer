package mana

import (
	"time"
)

// BaseMana is an interface for a collection of base mana values of a single node.
type BaseMana interface {
	update(time.Time) error
	revoke(float64, time.Time) error
	pledge(*TxInfo) float64
	BaseValue() float64
	EffectiveValue() float64
	LastUpdate() time.Time
}
