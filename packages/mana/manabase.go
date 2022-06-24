package mana

import (
	"github.com/iotaledger/hive.go/generics/model"
)

// ManaBase holds information about the consensus base mana values of a single node.
type ManaBase struct {
	model.Mutable[ManaBase, *ManaBase, ManaBaseModel] `serix:"0"`
}

type ManaBaseModel struct {
	BaseMana1 float64 `serix:"0"`
}

func NewManaBase(baseMana float64) *ManaBase {
	return model.NewMutable[ManaBase](&ManaBaseModel{BaseMana1: baseMana})
}

func (m *ManaBase) revoke(amount float64) error {
	m.Lock()
	defer m.Unlock()
	//if m.BaseMana1-amount < 0.0 {
	//	return ErrBaseManaNegative
	//}
	m.M.BaseMana1 -= amount
	return nil
}

func (m *ManaBase) pledge(pledgedAmount float64) {
	m.Lock()
	defer m.Unlock()
	m.M.BaseMana1 += pledgedAmount
}

// BaseValue returns the base mana value (BM1).
func (m *ManaBase) BaseValue() float64 {
	m.RLock()
	defer m.RUnlock()
	return m.M.BaseMana1
}

var _ BaseMana = &ManaBase{}
