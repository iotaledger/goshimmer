package protocol

import (
	"context"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type Settings struct {
	storage *settingsStorage

	sync.RWMutex
}

func NewSettings(filePath string) (settings *Settings) {
	return &Settings{
		storage: storable.InitStruct(&settingsStorage{
			Chains: set.NewAdvancedSet[commitment.ID](),
		}, filePath),
	}
}

func (s *Settings) ActiveChainID() commitment.ID {
	s.RLock()
	defer s.RUnlock()

	return s.storage.ActiveChainID
}

func (s *Settings) SetActiveChainID(chainID commitment.ID) {
	s.Lock()
	defer s.Unlock()

	s.storage.ActiveChainID = chainID
}

func (s *Settings) Chains() (chains *set.AdvancedSet[commitment.ID]) {
	s.RLock()
	defer s.RUnlock()

	return s.storage.Chains.Clone()
}

func (s *Settings) AddChain(chainID commitment.ID) (added bool) {
	s.Lock()
	defer s.Unlock()

	return s.storage.Chains.Add(chainID)
}

func (s *Settings) RemoveChain(chainID commitment.ID) (deleted bool) {
	s.Lock()
	defer s.Unlock()

	return s.storage.Chains.Delete(chainID)
}

func (s *Settings) Persist() {
	if err := s.storage.ToFile(); err != nil {
		panic(err)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region settingsStorage //////////////////////////////////////////////////////////////////////////////////////////////

type settingsStorage struct {
	ActiveChainID commitment.ID                   `serix:"0"`
	Chains        *set.AdvancedSet[commitment.ID] `serix:"1"`

	storable.Struct[settingsStorage, *settingsStorage]
}

func (s *settingsStorage) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s *settingsStorage) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), *s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
