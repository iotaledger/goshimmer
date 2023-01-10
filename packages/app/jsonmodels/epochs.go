package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Epoch struct {
	Index             uint64 `json:"index"`
	Commitment        string `json:"commitment"`
	StartTime         uint64 `json:"startTime"`
	EndTime           uint64 `json:"endTime"`
	Committed         bool   `json:"committed"`
	CommitmentRoot    string `json:"commitmentRoot"`
	PreviousRoot      string `json:"previousRoot"`
	NextRoot          string `json:"nextRoot"`
	TangleRoot        string `json:"tangleRoot"`
	StateMutationRoot string `json:"stateMutationRoot"`
	StateRoot         string `json:"stateRoot"`
	ManaRoot          string `json:"manaRoot"`
	CumulativeStake   string `json:"cumulativeStake"`
	Blocks            uint64 `json:"blocks"`
	Transactions      uint64 `json:"transactions"`
}

func EpochInfoFromRecord(c *commitment.Commitment) *Epoch {
	return &Epoch{
		Index:             uint64(c.Index()),
		Commitment:        "",
		StartTime:         0,
		EndTime:           0,
		Committed:         false,
		CommitmentRoot:    "",
		PreviousRoot:      "",
		NextRoot:          "",
		TangleRoot:        "",
		StateMutationRoot: "",
		StateRoot:         "",
		ManaRoot:          "",
		CumulativeStake:   "",
		Blocks:            0,
		Transactions:      0,
	}
}

type EpochsResponse struct {
	Epochs []*Epoch `json:"epochs"`
}

type EpochVotersWeightResponse struct {
	VotersWeight map[string]*NodeWeight `json:"voters"`
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
}

type EpochTransactionsResponse struct {
	Transactions []string `json:"transactions"`
}

type EpochPendingConflictCountResponse struct {
	PendingConflictCount uint64 `json:"pendingConflictCount"`
}
