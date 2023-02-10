package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type EpochInfo struct {
	ID               string `json:"id"`
	Index            uint64 `json:"index"`
	ECR              string `json:"rootsID"`
	PrevEC           string `json:"prevEC"`
	CumulativeWeight int64  `json:"cumulativeWeight"`
}

func EpochInfoFromRecord(c *commitment.Commitment) *EpochInfo {
	return &EpochInfo{
		ID:               c.ID().Base58(),
		Index:            uint64(c.Index()),
		ECR:              c.RootsID().Base58(),
		PrevEC:           c.PrevID().Base58(),
		CumulativeWeight: c.CumulativeWeight(),
	}
}

type EpochsResponse struct {
	Epochs []*EpochInfo `json:"epochs"`
}

type EpochVotersWeightResponse struct {
	VotersWeight map[string]*NodeWeight `json:"ecrVoters"`
}

type NodeWeight struct {
	Weights map[string]float64 `json:"weights"`
}

type EpochUTXOsResponse struct {
	SpentOutputs   []string `json:"spentOutputs"`
	CreatedOutputs []string `json:"createdOutputs"`
}

type EpochBlocksResponse struct {
	Blocks []string `json:"blocks"`
	Error  string   `json:"error,omitempty"`
}

type EpochTransactionsResponse struct {
	Transactions []string `json:"transactions"`
	Error        string   `json:"error,omitempty"`
}
