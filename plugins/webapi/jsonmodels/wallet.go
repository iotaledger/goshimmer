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
	Confirmed   bool `json:"confirmed"`
	Rejected    bool `json:"rejected"`
	Conflicting bool `json:"conflicting"`
}

// WalletOutputMetadata holds metadata about the output.
type WalletOutputMetadata struct {
	Timestamp time.Time `json:"timestamp"`
}
