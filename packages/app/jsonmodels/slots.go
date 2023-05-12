package jsonmodels

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
)

type SlotInfo struct {
	ID               string    `json:"id"`
	Index            uint64    `json:"index"`
	RootsID          string    `json:"rootsID"`
	PrevID           string    `json:"prevID"`
	CumulativeWeight int64     `json:"cumulativeWeight"`
	StartTimestamp   time.Time `json:"startTimestamp"`
	EndTimestamp     time.Time `json:"endTimestamp"`
}

func SlotInfoFromRecord(c *commitment.Commitment, start, end time.Time) *SlotInfo {
	return &SlotInfo{
		ID:               c.ID().Base58(),
		Index:            uint64(c.Index()),
		RootsID:          c.RootsID().Base58(),
		PrevID:           c.PrevID().Base58(),
		CumulativeWeight: c.CumulativeWeight(),
		StartTimestamp:   start,
		EndTimestamp:     end,
	}
}

type SlotsResponse struct {
	Slots []*SlotInfo `json:"slots"`
}

type SlotVotersWeightResponse struct {
	VotersWeight map[string]*NodeWeight `json:"voters"`
}

type NodeWeight struct {
	Weights map[string]float64 `json:"weights"`
}

type SlotUTXOsResponse struct {
	SpentOutputs   []string `json:"spentOutputs"`
	CreatedOutputs []string `json:"createdOutputs"`
}

type SlotBlocksResponse struct {
	Blocks []string `json:"blocks"`
	Error  string   `json:"error,omitempty"`
}

type SlotTransactionsResponse struct {
	Transactions []string `json:"transactions"`
	Error        string   `json:"error,omitempty"`
}

type LatestConfirmedIndexResponse struct {
	Index uint64 `json:"latestConfirmedIndex"`
	Error string `json:"error,omitempty"`
}

func CommitmentFromSlotInfo(s *SlotInfo) (*commitment.Commitment, error) {
	prevID := commitment.ID{}
	err := prevID.FromBase58(s.PrevID)
	if err != nil {
		return nil, err
	}
	rootsID := types.Identifier{}
	err = rootsID.FromBase58(s.RootsID)
	if err != nil {
		return nil, err
	}
	comm := commitment.New(slot.Index(s.Index), prevID, rootsID, s.CumulativeWeight)
	return comm, nil
}
