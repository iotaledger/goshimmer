package models

import "time"

type BlockVotePower struct {
	blockID BlockID
	time    time.Time
}

func NewBlockVotePower(id BlockID, time time.Time) BlockVotePower {
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

func (v BlockVotePower) Increase() BlockVotePower {
	return BlockVotePower{
		blockID: v.blockID,
		time:    v.time.Add(time.Nanosecond),
	}
}

func (v BlockVotePower) Decrease() BlockVotePower {
	return BlockVotePower{
		blockID: v.blockID,
		time:    v.time.Add(-time.Nanosecond),
	}
}
