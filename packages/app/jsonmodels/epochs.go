package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type EpochInfo struct {
	EI     uint64 `json:"EI"`
	ECR    string `json:"ECR"`
	PrevEC string `json:"prevEC"`
}

func EpochInfoFromRecord(record *epoch.ECRecord) *EpochInfo {
	return &EpochInfo{
		EI:     uint64(record.EI()),
		ECR:    record.ECR().Base58(),
		PrevEC: record.PrevEC().Base58(),
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
}

type EpochTransactionsResponse struct {
	Transactions []string `json:"transactions"`
}

type EpochPendingConflictCountResponse struct {
	PendingConflictCount uint64 `json:"pendingConflictCount"`
}
