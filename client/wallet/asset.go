package wallet

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Asset represents a container for all the information regarding a colored coin.
type Asset struct {
	// Color contains the identifier of this asset
	Color ledgerstate.Color

	// Name of the asset
	Name string

	// currency symbol of the asset (optional)
	Symbol string

	// Precision defines how many decimal places are shown when showing this asset in wallets
	Precision int

	// Address defines the target address where the asset is supposed to be created
	Address ledgerstate.Address

	// the amount of tokens that we want to create
	Amount uint64
}
