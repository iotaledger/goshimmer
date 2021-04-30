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
	Solid       bool `json:"solid"`
	Confirmed   bool `json:"confirmed"`
	Rejected    bool `json:"rejected"`
	Liked       bool `json:"liked"`
	Conflicting bool `json:"conflicting"`
	Finalized   bool `json:"finalized"`
	Preferred   bool `json:"preferred"`
}

// WalletOutputMetadata holds metadata about the output.
type WalletOutputMetadata struct {
	Timestamp time.Time `json:"timestamp"`
}
