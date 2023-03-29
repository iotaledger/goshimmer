package booker

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type BlockVotePower struct {
	blockID models.BlockID
	time    time.Time
}

func NewBlockVotePower(id models.BlockID, time time.Time) BlockVotePower {
	return BlockVotePower{
		blockID: id,
		time:    time,
	}
}

func (v BlockVotePower) Compare(other BlockVotePower) int {
	if v.time.Before(other.time) {
		return -1
	} else if v.time.After(other.time) {
		return 1
	} else {
		return v.blockID.CompareTo(other.blockID)
	}
}
