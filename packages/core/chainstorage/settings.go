package chainstorage

import (
	"context"
	"sync"

	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type settings struct {
	LatestCommittedEpoch epoch.Index `serix:"0"`
	LatestAcceptedEpoch  epoch.Index `serix:"1"`
	LatestConfirmedEpoch epoch.Index `serix:"2"`

	storable.Struct[settings, *settings]

	rwMutex
}

func (s *settings) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s *settings) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), *s)
}

type rwMutex = sync.RWMutex

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
