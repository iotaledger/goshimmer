package jsonmodels

import "github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"

// GlobalMetricsResponse contains global metrics for explorer.
type GlobalMetricsResponse struct {
	BPS                uint64  `json:"bps"`
	BookedTransactions uint64  `json:"bookedTransactions"`
	InclusionRate      float64 `json:"inclusionRate"`
	ConfirmationDelay  string  `json:"confirmationDelay"`
	ActiveManaRatio    float64 `json:"activeManaRatio"`
	OnlineNodes        int     `json:"onlineNodes"`
	ConflictsResolved  uint64  `json:"conflictsResolved"`
	TotalConflicts     uint64  `json:"totalConflicts"`
}

// NodesMetricsResponse contains metrics of all nodes in the network.
type NodesMetricsResponse struct {
	Cmanas          []manamodels.IssuerStr `json:"cmanas"`
	ActiveManaRatio float64                `json:"activeManaRatio"`
	OnlineNodes     int                    `json:"onlineNodes"`
	MaxBPS          float64                `json:"maxBPS"`
	BlockScheduled  uint64                 `json:"blockscheduled"`
}
