package jsonmodels

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
)

// GetManaRequest is the request for get mana.
type GetManaRequest struct {
	IssuerID string `json:"nodeID"`
}

// GetManaResponse defines the response for get mana.
type GetManaResponse struct {
	Error              string `json:"error,omitempty"`
	ShortIssuerID      string `json:"shortNodeID"`
	IssuerID           string `json:"nodeID"`
	Access             int64  `json:"access"`
	AccessTimestamp    int64  `json:"accessTimestamp"`
	Consensus          int64  `json:"consensus"`
	ConsensusTimestamp int64  `json:"consensusTimestamp"`
}

// GetAllManaResponse is the request to a getAllManaHandler request.
type GetAllManaResponse struct {
	Access             []manamodels.IssuerStr `json:"access"`
	AccessTimestamp    int64                  `json:"accessTimestamp"`
	Consensus          []manamodels.IssuerStr `json:"consensus"`
	ConsensusTimestamp int64                  `json:"consensusTimestamp"`
	Error              string                 `json:"error,omitempty"`
}

// GetEventLogsRequest is the request.
type GetEventLogsRequest struct {
	IssuerIDs []string `json:"nodeIDs"`
	StartTime int64    `json:"startTime"`
	EndTime   int64    `json:"endTime"`
}

// GetEventLogsResponse is the response.
type GetEventLogsResponse struct {
	Error     string `json:"error,omitempty"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// GetNHighestResponse holds info about nodes and their mana values.
type GetNHighestResponse struct {
	Error     string                 `json:"error,omitempty"`
	Issuers   []manamodels.IssuerStr `json:"nodes,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// GetOnlineResponse is the response to an online mana request.
type GetOnlineResponse struct {
	Online    []*OnlineIssuerStr `json:"online"`
	Error     string             `json:"error,omitempty"`
	Timestamp int64              `json:"timestamp"`
}

// OnlineIssuerStr holds information about online rank, nodeID and mana.
type OnlineIssuerStr struct {
	OnlineRank int    `json:"rank"`
	ShortID    string `json:"shortNodeID"`
	ID         string `json:"nodeID"`
	Mana       int64  `json:"mana"`
}

// PastConsensusManaVectorResponse is the response.
type PastConsensusManaVectorResponse struct {
	Consensus []manamodels.IssuerStr `json:"consensus"`
	Error     string                 `json:"error,omitempty"`
	TimeStamp int64                  `json:"timestamp"`
}

// GetPercentileRequest is the request object of mana/percentile.
type GetPercentileRequest struct {
	IssuerID string `json:"nodeID"`
}

// GetPercentileResponse holds info about the mana percentile(s) of a node.
type GetPercentileResponse struct {
	Error              string  `json:"error,omitempty"`
	ShortIssuerID      string  `json:"shortNodeID"`
	IssuerID           string  `json:"nodeID"`
	Access             float64 `json:"access"`
	AccessTimestamp    int64   `json:"accessTimestamp"`
	Consensus          float64 `json:"consensus"`
	ConsensusTimestamp int64   `json:"consensusTimestamp"`
}

// AllowedManaPledgeResponse is the http response.
type AllowedManaPledgeResponse struct {
	Access    AllowedPledge `json:"accessMana"`
	Consensus AllowedPledge `json:"consensusMana"`
	Error     string        `json:"error,omitempty"`
}

// AllowedPledge represents the nodes that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool     `json:"isFilterEnabled"`
	Allowed         []string `json:"allowed,omitempty"`
}
