package jsonmodels

import "time"

// InfoResponse holds the response of the GET request.
type InfoResponse struct {
	// version of GoShimmer
	Version string `json:"version,omitempty"`
	// Network Version of the autopeering
	NetworkVersion uint32 `json:"networkVersion,omitempty"`
	// whether the node is synchronized
	Synced bool `json:"synced"`
	// sync beacons status
	Beacons []Beacon `json:"beacons"`
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
	// ManaDecay is the decay coefficient of bm2.
	ManaDecay float64 `json:"mana_decay"`
	// error of the response
	Error string `json:"error,omitempty"`
}

// Beacon contains a sync beacons detailed status.
type Beacon struct {
	PublicKey string `json:"public_key"`
	MsgID     string `json:"msg_id"`
	SentTime  int64  `json:"sent_time"`
	Synced    bool   `json:"synced"`
}

// Mana contains the different mana values of the node.
type Mana struct {
	Access             float64   `json:"access"`
	AccessTimestamp    time.Time `json:"accessTimestamp"`
	Consensus          float64   `json:"consensus"`
	ConsensusTimestamp time.Time `json:"consensusTimestamp"`
}
