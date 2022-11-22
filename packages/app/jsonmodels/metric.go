package jsonmodels

import "github.com/iotaledger/goshimmer/packages/protocol/engine/manatracker/manamodels"

// GlobalMetricsResponse contains global metrics for explorer.
type GlobalMetricsResponse struct {
	BlockStored       uint64  `json:"blockStored"`
	InclusionRate     float64 `json:"inclusionRate"`
	ConfirmationDelay string  `json:"confirmationDelay"`
	ActiveManaRatio   float64 `json:"activeManaRatio"`
	OnlineNodes       int     `json:"onlineNodes"`
	ConflictsResolved uint64  `json:"conflictsResolved"`
	TotalConflicts    uint64  `json:"totalConflicts"`
}

// NodesMetricsResponse contains metrics of all nodes in the network.
type NodesMetricsResponse struct {
	Cmanas          []manamodels.IssuerStr `json:"cmanas"`
	ActiveManaRatio float64                `json:"activeManaRatio"`
	OnlineNodes     int                    `json:"onlineNodes"`
}
