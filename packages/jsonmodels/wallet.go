package jsonmodels

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
)

// WalletOutputsOnAddress represents wallet outputs on an address.
type WalletOutputsOnAddress struct {
	Address Address        `json:"address"`
	Outputs []WalletOutput `json:"outputs"`
}

// WalletOutput represents an output as expected by the wallet lib.
type WalletOutput struct {
	Output          Output               `json:"output"`
	Metadata        WalletOutputMetadata `json:"metadata"`
	GradeOfFinality gof.GradeOfFinality  `json:"gradeOfFinality"`
}

// WalletOutputMetadata holds metadata about the output.
type WalletOutputMetadata struct {
	Timestamp time.Time `json:"timestamp"`
}
