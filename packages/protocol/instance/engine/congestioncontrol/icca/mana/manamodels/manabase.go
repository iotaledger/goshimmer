package manamodels

import (
	"github.com/iotaledger/hive.go/core/generics/model"
)

// ManaBase holds information about the consensus base mana values of a single node.
type ManaBase struct {
	model.Mutable[ManaBase, *ManaBase, manaBaseModel] `serix:"0"`
}

type manaBaseModel struct {
	Value int64 `serix:"0"`
}

func NewManaBase(value int64) *ManaBase {
	return model.NewMutable[ManaBase](&manaBaseModel{Value: value})
}

func (m *ManaBase) revoke(amount int64) error {
	m.Lock()
	defer m.Unlock()
	//if m.BaseMana1-amount < 0.0 {
	//	return ErrBaseManaNegative
	//}
	m.M.Value -= amount
	return nil
}

func (m *ManaBase) pledge(pledgedAmount int64) {
	m.Lock()
	defer m.Unlock()
	m.M.Value += pledgedAmount
}

// BaseValue returns the base mana value (BM1).
func (m *ManaBase) BaseValue() int64 {
	m.RLock()
	defer m.RUnlock()
	return m.M.Value
}

var _ BaseMana = &ManaBase{}
