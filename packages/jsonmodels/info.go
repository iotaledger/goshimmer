package jsonmodels

import (
	"time"
)

// InfoResponse holds the response of the GET request.
type InfoResponse struct {
	// version of GoShimmer
	Version string `json:"version,omitempty"`
	// Network Version of the autopeering
	NetworkVersion uint32 `json:"networkVersion,omitempty"`
	// TangleTime sync status
	TangleTime TangleTime `json:"tangleTime,omitempty"`
	// identity ID of the node encoded in base58
	IdentityID string `json:"identityID,omitempty"`
	// identity ID of the node encoded in base58 and truncated to its first 8 bytes
	IdentityIDShort string `json:"identityIDShort,omitempty"`
	// public key of the node encoded in base58
	PublicKey string `json:"publicKey,omitempty"`
	// MessageRequestQueueSize is the number of messages a node is trying to request from neighbors.
	MessageRequestQueueSize int `json:"messageRequestQueueSize,omitempty"`
	// SolidMessageCount is the number of solid messages in the node's database.
	SolidMessageCount int `json:"solidMessageCount,omitempty"`
	// TotalMessageCount is the number of messages in the node's database.
	TotalMessageCount int `json:"totalMessageCount,omitempty"`
	// list of enabled plugins
	EnabledPlugins []string `json:"enabledPlugins,omitempty"`
	// list if disabled plugins
	DisabledPlugins []string `json:"disabledPlugins,omitempty"`
	// Mana values
	Mana Mana `json:"mana,omitempty"`
	// Mana Delegation Address
	ManaDelegationAddress string `json:"manaDelegationAddress,omitempty"`
	// ManaDecay is the decay coefficient of bm2.
	ManaDecay float64 `json:"mana_decay"`
	// Scheduler is the scheduler.
	Scheduler Scheduler `json:"scheduler"`
	// error of the response
	Error string `json:"error,omitempty"`
}

// TangleTime contains the TangleTime sync detailed status.
type TangleTime struct {
	MessageID string `json:"messageID"`
	Time      int64  `json:"time"`
	Synced    bool   `json:"synced"`
}

// Mana contains the different mana values of the node.
type Mana struct {
	Access             float64   `json:"access"`
	AccessTimestamp    time.Time `json:"accessTimestamp"`
	Consensus          float64   `json:"consensus"`
	ConsensusTimestamp time.Time `json:"consensusTimestamp"`
}

// Scheduler is the scheduler details.
type Scheduler struct {
	Running           bool           `json:"running"`
	Rate              string         `json:"rate"`
	MaxBufferSize     int            `json:"maxBufferSize"`
	CurrentBufferSize int            `json:"currentBufferSizer"`
	NodeQueueSizes    map[string]int `json:"nodeQueueSizes"`
}

// RateSetter is the rate setter details.
type RateSetter struct {
	Rate float64 `json:"rate"`
	Size int     `json:"size"`
}
