package chainstorage

import (
	"context"
	"sync"

	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type Settings struct {
	LatestCommitment         *commitment.Commitment `serix:"0"`
	LatestStateMutationEpoch epoch.Index            `serix:"1"`
	LatestConfirmedEpoch     epoch.Index            `serix:"2"`
	Chain                    commitment.ID          `serix:"3"`

	storable.Struct[Settings, *Settings]
	sync.RWMutex
}

func (s *Settings) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, s)
}

func (s Settings) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), s)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
