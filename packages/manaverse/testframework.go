package manaverse

import (
	"sync"

	"github.com/iotaledger/hive.go/identity"
)

type MockedManaLedger struct {
	manaBalances      map[identity.ID]int64
	manaBalancesMutex sync.Mutex
}

func NewMockedManaLedger() (newMockedManaLedger *MockedManaLedger) {
	return &MockedManaLedger{
		manaBalances: make(map[identity.ID]int64),
	}
}

func (m *MockedManaLedger) IncreaseMana(id identity.ID, mana int64) (newBalance int64) {
	m.manaBalancesMutex.Lock()
	defer m.manaBalancesMutex.Unlock()

	m.manaBalances[id] += mana

	return m.manaBalances[id]
}

func (m *MockedManaLedger) DecreaseMana(id identity.ID, mana int64) (newBalance int64) {
	m.manaBalancesMutex.Lock()
	defer m.manaBalancesMutex.Unlock()

	m.manaBalances[id] -= mana

	return m.manaBalances[id]
}

var _ ManaLedger = new(MockedManaLedger)
