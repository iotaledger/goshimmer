package jsonmodels

import (
	"github.com/iotaledger/hive.go/identity"
	"time"

	"github.com/iotaledger/goshimmer/packages/mana"
)

// Epoch represents the JSON model of epochs.Epoch.
type Epoch struct {
	EpochID        uint64         `json:"epochID"`
	EpochStartTime int64          `json:"epochStartTime"`
	EpochEndTime   int64          `json:"epochEndTime"`
	Weights        []mana.NodeStr `json:"weights"`
	TotalWeight    float64        `json:"totalWeight"`
}

// NewEpoch is the constructor for Epoch.
func NewEpoch(epochID uint64, epochStartTime, epochEndTime time.Time, weights map[identity.ID]float64, totalWeight float64) (epoch Epoch) {
	epoch.EpochID = epochID
	epoch.EpochStartTime = epochStartTime.Unix()
	epoch.EpochEndTime = epochEndTime.Unix()
	epoch.TotalWeight = totalWeight

	for nodeID, weight := range weights {
		node := mana.Node{
			ID:   nodeID,
			Mana: weight,
		}
		epoch.Weights = append(epoch.Weights, node.ToNodeStr())
	}

	return
}

// EpochID represents the JSON model of epochs.ID.
type EpochID struct {
	EpochID uint64 `json:"epochID"`
}
