package protocol

import (
	"context"
	"sync"

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
		storage: storable.InitStruct(&settingsStorage{}, filePath),
	}
}

func (s *Settings) ChainID() commitment.ID {
	s.RLock()
	defer s.RUnlock()

	return s.storage.ChainID
}

func (s *Settings) SetChainID(chainID commitment.ID) {
	s.Lock()
	defer s.Unlock()

	s.storage.ChainID = chainID

	s.Persist()
}

func (s *Settings) SnapshotChecksum() (checksum [32]byte) {
	s.RLock()
	defer s.RUnlock()

	return s.storage.SnapshotChecksum
}

func (s *Settings) Persist() {
	if err := s.storage.ToFile(); err != nil {
		panic(err)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region settingsStorage //////////////////////////////////////////////////////////////////////////////////////////////

type settingsStorage struct {
	ChainID          commitment.ID `serix:"0"`
	SnapshotChecksum [32]byte      `serix:"1"`

	storable.Struct[settingsStorage, *settingsStorage]
}

func (s *settingsStorage) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s *settingsStorage) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), *s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
