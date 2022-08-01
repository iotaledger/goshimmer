package mana

import (
	"github.com/iotaledger/hive.go/core/generics/model"
)

// ManaBase holds information about the consensus base mana values of a single node.
type ManaBase struct {
	model.Mutable[ManaBase, *ManaBase, manaBaseModel] `serix:"0"`
}

type manaBaseModel struct {
	Value float64 `serix:"0"`
}

func NewManaBase(value float64) *ManaBase {
	return model.NewMutable[ManaBase](&manaBaseModel{Value: value})
}

func (m *ManaBase) revoke(amount float64) error {
	m.Lock()
	defer m.Unlock()
	//if m.BaseMana1-amount < 0.0 {
	//	return ErrBaseManaNegative
	//}
	m.M.Value -= amount
	return nil
}

func (m *ManaBase) pledge(pledgedAmount float64) {
	m.Lock()
	defer m.Unlock()
	m.M.Value += pledgedAmount
}

// BaseValue returns the base mana value (BM1).
func (m *ManaBase) BaseValue() float64 {
	m.RLock()
	defer m.RUnlock()
	return m.M.Value
}

var _ BaseMana = &ManaBase{}
