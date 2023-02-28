package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type SlotInfo struct {
	ID               string `json:"id"`
	Index            uint64 `json:"index"`
	RootsID          string `json:"rootsID"`
	PrevID           string `json:"prevID"`
	CumulativeWeight int64  `json:"cumulativeWeight"`
}

func SlotInfoFromRecord(c *commitment.Commitment) *SlotInfo {
	return &SlotInfo{
		ID:               c.ID().Base58(),
		Index:            uint64(c.Index()),
		RootsID:          c.RootsID().Base58(),
		PrevID:           c.PrevID().Base58(),
		CumulativeWeight: c.CumulativeWeight(),
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
