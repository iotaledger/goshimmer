package jsonmodels

import "time"

// WalletOutputsOnAddress represents wallet outputs on an address.
type WalletOutputsOnAddress struct {
	Address Address        `json:"address"`
	Outputs []WalletOutput `json:"outputs"`
}

// WalletOutput represents an output as expected by the wallet lib.
type WalletOutput struct {
	Output         Output               `json:"output"`
	InclusionState InclusionState       `json:"inclusionState"`
	Metadata       WalletOutputMetadata `json:"metadata"`
}

// InclusionState represents the different states of an output.
type InclusionState struct {
	Solid       bool `json:"solid,omitempty"`
	Confirmed   bool `json:"confirmed,omitempty"`
	Rejected    bool `json:"rejected,omitempty"`
	Liked       bool `json:"liked,omitempty"`
	Conflicting bool `json:"conflicting,omitempty"`
	Finalized   bool `json:"finalized,omitempty"`
	Preferred   bool `json:"preferred,omitempty"`
}

// WalletOutputMetadata holds metadata about the output.
type WalletOutputMetadata struct {
	Timestamp time.Time `json:"timestamp"`
}
