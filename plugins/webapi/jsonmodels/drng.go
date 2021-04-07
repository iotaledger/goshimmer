package jsonmodels

import "time"

// CollectiveBeaconResponse is the HTTP response from broadcasting a collective beacon message.
type CollectiveBeaconResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// CollectiveBeaconRequest is a request containing a collective beacon response.
type CollectiveBeaconRequest struct {
	Payload []byte `json:"payload"`
}

// CommitteeResponse is the HTTP message containing the DRNG committee.
type CommitteeResponse struct {
	Committees []Committee `json:"committees,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Committee defines the information about a committee.
type Committee struct {
	InstanceID    uint32   `json:"instanceID,omitempty"`
	Threshold     uint8    `json:"threshold,omitempty"`
	Identities    []string `json:"identities,omitempty"`
	DistributedPK string   `json:"distributedPK,omitempty"`
}

// RandomnessResponse is the HTTP message containing the current DRNG randomness.
type RandomnessResponse struct {
	Randomness []Randomness `json:"randomness,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// Randomness defines the content of new randomness.
type Randomness struct {
	InstanceID uint32    `json:"instanceID,omitempty"`
	Round      uint64    `json:"round,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Randomness []byte    `json:"randomness,omitempty"`
}
