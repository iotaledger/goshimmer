package mana

import (
	"github.com/iotaledger/hive.go/identity"
)

// BaseManaVector represents a base mana vector
type BaseManaVector struct {
	vector     map[identity.ID]*BaseMana
	vectorType Type
}

// NewBaseManaVector creates and returns a new base mana vector for the specified type
func NewBaseManaVector(vectorType Type) *BaseManaVector {
	return &BaseManaVector{
		vector:     make(map[identity.ID]*BaseMana),
		vectorType: vectorType,
	}
}

// TODO: update
// TODO: bookmana, refactor methods pass mana type

// BookMana books mana for a certain node
func (bmv *BaseManaVector) BookMana(txInfo *TxInfo) {

}
