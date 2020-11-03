package wallet

import (
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// Output is a wallet specific representation of an output in the IOTA network.
type Output struct {
	ID             transaction.OutputID
	Address        address.Address
	TransactionID  transaction.ID
	Balances       map[balance.Color]uint64
	InclusionState InclusionState
	Metadata       OutputMetadata
}

// InclusionState is a container for the different flags of an output that define if it was accepted in the network.
type InclusionState struct {
	Liked       bool
	Confirmed   bool
	Rejected    bool
	Conflicting bool
	Spent       bool
}

// OutputMetadata is metadata about the output.
type OutputMetadata struct {
	// Timestamp is the timestamp of the tx that created the output.
	Timestamp time.Time
}
