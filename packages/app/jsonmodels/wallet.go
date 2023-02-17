package jsonmodels

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
)

// WalletOutputsOnAddress represents wallet outputs on an address.
type WalletOutputsOnAddress struct {
	Address Address        `json:"address"`
	Outputs []WalletOutput `json:"outputs"`
}

// WalletOutput represents an output as expected by the wallet lib.
type WalletOutput struct {
	Output            Output               `json:"output"`
	Metadata          WalletOutputMetadata `json:"metadata"`
	ConfirmationState confirmation.State   `json:"confirmationState"`
}

// WalletOutputMetadata holds metadata about the output.
type WalletOutputMetadata struct {
	Timestamp time.Time `json:"timestamp"`
}
