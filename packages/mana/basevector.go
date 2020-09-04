package mana

import (
	"github.com/iotaledger/hive.go/identity"
)

type BaseManaVector struct {
	vector     map[identity.ID]*BaseMana
	vectorType Type
}

// TODO: update
// TODO: bookmana, refactor methods pass mana type

func (bmv *BaseManaVector) bookMana(txInfo *TxInfo) {

}
