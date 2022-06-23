package jsonmodels

import (
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/epoch"
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
	VotersWeight map[identity.ID]float64 `json:"votersWeight"`
}

type EpochUTXOsResponse struct {
	SpentOutputs   []string `json:"spentOutputs"`
	CreatedOutputs []string `json:"createdOutputs"`
}

type EpochMessagesResponse struct {
	Messages []string `json:"messages"`
}

type EpochTransactionsResponse struct {
	Transactions []string `json:"transactions"`
}

type EpochPendingBranchCountResponse struct {
	PendingBranchCount uint64 `json:"pendingBranchCount"`
}
