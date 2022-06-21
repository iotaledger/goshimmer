package manaverse

import (
	"github.com/iotaledger/hive.go/identity"
)

type ManaLedger interface {
	IncreaseMana(id identity.ID, mana int64) (newBalance int64)
	DecreaseMana(id identity.ID, mana int64) (newBalance int64)
}
