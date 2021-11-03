package jsonmodels

import "github.com/iotaledger/goshimmer/packages/mana"

// GetManaRequest is the request for get mana.
type GetManaRequest struct {
	NodeID string `json:"nodeID"`
}

// GetManaResponse defines the response for get mana.
type GetManaResponse struct {
	Error              string  `json:"error,omitempty"`
	ShortNodeID        string  `json:"shortNodeID"`
	NodeID             string  `json:"nodeID"`
	Access             float64 `json:"access"`
	AccessTimestamp    int64   `json:"accessTimestamp"`
	Consensus          float64 `json:"consensus"`
	ConsensusTimestamp int64   `json:"consensusTimestamp"`
}

// GetAllManaResponse is the request to a getAllManaHandler request.
type GetAllManaResponse struct {
	Access             []mana.NodeStr `json:"access"`
	AccessTimestamp    int64          `json:"accessTimestamp"`
	Consensus          []mana.NodeStr `json:"consensus"`
	ConsensusTimestamp int64          `json:"consensusTimestamp"`
	Error              string         `json:"error,omitempty"`
}

// EventLogsJSON is a events log in JSON.
type EventLogsJSON struct {
	Pledge []*mana.PledgedEventJSON `json:"pledge"`
	Revoke []*mana.RevokedEventJSON `json:"revoke"`
}

// GetEventLogsRequest is the request.
type GetEventLogsRequest struct {
	NodeIDs   []string `json:"nodeIDs"`
	StartTime int64    `json:"startTime"`
	EndTime   int64    `json:"endTime"`
}

// GetEventLogsResponse is the response.
type GetEventLogsResponse struct {
	Logs      map[string]*EventLogsJSON `json:"logs"`
	Error     string                    `json:"error,omitempty"`
	StartTime int64                     `json:"startTime"`
	EndTime   int64                     `json:"endTime"`
}

// GetNHighestResponse holds info about nodes and their mana values.
type GetNHighestResponse struct {
	Error     string         `json:"error,omitempty"`
	Nodes     []mana.NodeStr `json:"nodes,omitempty"`
	Timestamp int64          `json:"timestamp"`
}

// GetOnlineResponse is the response to an online mana request.
type GetOnlineResponse struct {
	Online    []OnlineNodeStr `json:"online"`
	Error     string          `json:"error,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// OnlineNodeStr holds information about online rank, nodeID and mana.
type OnlineNodeStr struct {
	OnlineRank int     `json:"rank"`
	ShortID    string  `json:"shortNodeID"`
	ID         string  `json:"nodeID"`
	Mana       float64 `json:"mana"`
}

// PastConsensusVectorMetadataResponse is the response.
type PastConsensusVectorMetadataResponse struct {
	Metadata *mana.ConsensusBasePastManaVectorMetadata `json:"metadata,omitempty"`
	Error    string                                    `json:"error,omitempty"`
}

// PastConsensusManaVectorRequest is the request.
type PastConsensusManaVectorRequest struct {
	Timestamp int64 `json:"timestamp"`
}

// PastConsensusManaVectorResponse is the response.
type PastConsensusManaVectorResponse struct {
	Consensus []mana.NodeStr `json:"consensus"`
	Error     string         `json:"error,omitempty"`
	TimeStamp int64          `json:"timestamp"`
}

// PendingRequest is the pending mana request.
type PendingRequest struct {
	OutputID string `json:"outputID"`
}

// PendingResponse is the pending mana response.
type PendingResponse struct {
	Mana      float64 `json:"mana"`
	OutputID  string  `json:"outputID"`
	Error     string  `json:"error,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

// GetPercentileRequest is the request object of mana/percentile.
type GetPercentileRequest struct {
	NodeID string `json:"nodeID"`
}

// GetPercentileResponse holds info about the mana percentile(s) of a node.
type GetPercentileResponse struct {
	Error              string  `json:"error,omitempty"`
	ShortNodeID        string  `json:"shortNodeID"`
	NodeID             string  `json:"nodeID"`
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
