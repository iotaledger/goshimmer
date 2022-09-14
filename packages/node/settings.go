package node

import (
	"context"
	"sync"

	"github.com/iotaledger/hive.go/core/serix"

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
			LatestCleanEpoch: 0,
		}, filePath),
	}
}

func (s *Settings) LatestCleanEpoch() uint64 {
	s.RLock()
	defer s.RUnlock()

	return s.storage.LatestCleanEpoch
}

func (s *Settings) SetLatestCleanEpoch(epoch uint64) {
	s.Lock()
	defer s.Unlock()

	s.storage.LatestCleanEpoch = epoch

	s.Persist()
}

func (s *Settings) Persist() {
	if err := s.storage.ToFile(); err != nil {
		panic(err)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region settingsStorage //////////////////////////////////////////////////////////////////////////////////////////////

type settingsStorage struct {
	LatestCleanEpoch uint64 `serix:"1"`

	storable.Struct[settingsStorage, *settingsStorage]
}

func (s *settingsStorage) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s *settingsStorage) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), *s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
