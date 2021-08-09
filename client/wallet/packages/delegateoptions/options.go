package delegateoptions

import (
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/constants"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// DelegateFundsOption is the type for the optional parameters for the DelegateFunds call.
type DelegateFundsOption func(*DelegateFundsOptions) error

// Destination is an option for the SendFunds call that defines a destination for funds that are supposed to be moved.
func Destination(addr address.Address, balance map[ledgerstate.Color]uint64) DelegateFundsOption {
	// return an error if the IOTA amount is less
	if balance[ledgerstate.ColorIOTA] < ledgerstate.DustThresholdAliasOutputIOTA {
		return optionError(errors.Errorf("the IOTA amount provided in the destination needs to be larger than %d", ledgerstate.DustThresholdAliasOutputIOTA))
	}

	// return Option
	return func(options *DelegateFundsOptions) error {
		// initialize destinations property
		if options.Destinations == nil {
			options.Destinations = make(map[address.Address]map[ledgerstate.Color]uint64)
		}

		// initialize address specific destination
		if _, addressExists := options.Destinations[addr]; !addressExists {
			options.Destinations[addr] = make(map[ledgerstate.Color]uint64)
		}

		for color, amount := range balance {
			// increase amount
			options.Destinations[addr][color] += amount
		}
		return nil
	}
}

// DelegateUntil is an option for the DelegateFunds call that specifies until when the delegation should last.
func DelegateUntil(until time.Time) DelegateFundsOption {
	return func(options *DelegateFundsOptions) error {
		if until.Before(time.Now()) {
			return errors.Errorf("can't delegate funds in the past")
		}
		if until.After(constants.MaxRepresentableTime) {
			return errors.Errorf("delegation is only supported until %s", constants.MaxRepresentableTime)
		}
		options.DelegateUntil = until
		return nil
	}
}

// Remainder is an option for the SendsFunds call that allows us to specify the remainder address that is
// supposed to be used in the corresponding transaction.
func Remainder(addr address.Address) DelegateFundsOption {
	return func(options *DelegateFundsOptions) error {
		options.RemainderAddress = addr

		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) DelegateFundsOption {
	return func(options *DelegateFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) DelegateFundsOption {
	return func(options *DelegateFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) DelegateFundsOption {
	return func(options *DelegateFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// DelegateFundsOptions is a struct that is used to aggregate the optional parameters provided in the DelegateFunds call.
type DelegateFundsOptions struct {
	Destinations          map[address.Address]map[ledgerstate.Color]uint64
	DelegateUntil         time.Time
	RemainderAddress      address.Address
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// RequiredFunds derives how much funds are needed based on the Destinations to fund the transfer.
func (s *DelegateFundsOptions) RequiredFunds() map[ledgerstate.Color]uint64 {
	// aggregate total amount of required funds, so we now what and how many funds we need
	requiredFunds := make(map[ledgerstate.Color]uint64)
	for _, coloredBalances := range s.Destinations {
		for color, amount := range coloredBalances {
			// if we want to color sth then we need fresh IOTA
			if color == ledgerstate.ColorMint {
				color = ledgerstate.ColorIOTA
			}

			requiredFunds[color] += amount
		}
	}
	return requiredFunds
}

// Build is a utility function that constructs the DelegateFundsOptions.
func Build(options ...DelegateFundsOption) (result *DelegateFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &DelegateFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	// sanitize parameters
	if len(result.Destinations) == 0 {
		err = errors.New("you need to provide at least one Destination for a valid delegation to be issued")

		return
	}

	return
}

// optionError is a utility function that returns a Option that returns the error provided in the
// argument.
func optionError(err error) DelegateFundsOption {
	return func(options *DelegateFundsOptions) error {
		return err
	}
}
