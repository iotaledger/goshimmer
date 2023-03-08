package delegateoptions

import (
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// DelegateFundsOptions is a struct that is used to aggregate the optional parameters provided in the DelegateFunds call.
type DelegateFundsOptions struct {
	Destinations          map[address.Address]map[devnetvm.Color]uint64
	DelegateUntil         time.Time
	RemainderAddress      address.Address
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// RequiredFunds derives how much funds are needed based on the Destinations to fund the transfer.
func (s *DelegateFundsOptions) RequiredFunds() map[devnetvm.Color]uint64 {
	// aggregate total amount of required funds, so we now what and how many funds we need
	requiredFunds := make(map[devnetvm.Color]uint64)
	for _, coloredBalances := range s.Destinations {
		for color, amount := range coloredBalances {
			// if we want to color sth then we need fresh IOTA
			if color == devnetvm.ColorMint {
				color = devnetvm.ColorIOTA
			}

			requiredFunds[color] += amount
		}
	}
	return requiredFunds
}
