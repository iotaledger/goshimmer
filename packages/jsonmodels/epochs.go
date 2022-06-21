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

func EpochInfoFromRecord(record epoch.ECRecord) EpochInfo {
	return EpochInfo{
		EI:     uint64(record.EI()),
		ECR:    record.ECR().String(),
		PrevEC: record.PrevEC().String(),
	}
}

type EpochVotersWeightResponse struct {
	VotersWeight map[identity.ID]float64 `json:"votersWeight"`
}

type EpochUTXOsResponse struct {
	UTXOs []string `json:"UTXOs"`
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
