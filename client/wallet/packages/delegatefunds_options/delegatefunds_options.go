package delegatefunds_options

import (
	"errors"
	"golang.org/x/xerrors"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// DelegateFundsOption is the type for the optional parameters for the DelegateFunds call.
type DelegateFundsOption func(*delegateFundsOptions) error

// Destination is an option for the SendFunds call that defines a destination for funds that are supposed to be moved.
func Destination(addr address.Address, amount uint64, optionalColor ...ledgerstate.Color) DelegateFundsOption {
	// determine optional output color
	var outputColor ledgerstate.Color
	switch len(optionalColor) {
	case 0:
		outputColor = ledgerstate.ColorIOTA
	case 1:
		outputColor = optionalColor[0]
	default:
		return optionError(xerrors.Errorf("providing more than one output color for the destination of funds is forbidden"))
	}

	// return an error if the amount is less
	if amount < ledgerstate.DustThresholdAliasOutputIOTA {
		return optionError(xerrors.Errorf("the amount provided in the destinations needs to be larger than %d", ledgerstate.DustThresholdAliasOutputIOTA))
	}

	// return Option
	return func(options *delegateFundsOptions) error {
		// initialize destinations property
		if options.Destinations == nil {
			options.Destinations = make(map[address.Address]map[ledgerstate.Color]uint64)
		}

		// initialize address specific destination
		if _, addressExists := options.Destinations[addr]; !addressExists {
			options.Destinations[addr] = make(map[ledgerstate.Color]uint64)
		}

		// initialize color specific destination
		if _, colorExists := options.Destinations[addr][outputColor]; !colorExists {
			options.Destinations[addr][outputColor] = 0
		}

		// increase amount
		options.Destinations[addr][outputColor] += amount

		return nil
	}
}

// DelegateUntil is an option for the DelegateFunds call that specifies until when the delegation should last.
func DelegateUntil(until time.Time) DelegateFundsOption {
	return func(options *delegateFundsOptions) error {
		if until.Before(time.Now()) {
			return xerrors.Errorf("can't delegate funds in the past")
		}
		options.DelegateUntil = until
		return nil
	}
}

// Remainder is an option for the SendsFunds call that allows us to specify the remainder address that is
// supposed to be used in the corresponding transaction.
func Remainder(addr address.Address) DelegateFundsOption {
	return func(options *delegateFundsOptions) error {
		options.RemainderAddress = addr

		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) DelegateFundsOption {
	return func(options *delegateFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) DelegateFundsOption {
	return func(options *delegateFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// WaitForConfirmation
func WaitForConfirmation(wait bool) DelegateFundsOption {
	return func(options *delegateFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// delegateFundsOptions is a struct that is used to aggregate the optional parameters provided in the DelegateFunds call.
type delegateFundsOptions struct {
	Destinations          map[address.Address]map[ledgerstate.Color]uint64
	DelegateUntil         time.Time
	RemainderAddress      address.Address
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// RequiredFunds derives how much funds are needed based on the Destinations to fund the transfer.
func (s *delegateFundsOptions) RequiredFunds() map[ledgerstate.Color]uint64 {
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

// BuildDelegateFundsOptions is a utility function that constructs the delegateFundsOptions.
func BuildDelegateFundsOptions(options ...DelegateFundsOption) (result *delegateFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &delegateFundsOptions{}

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
	return func(options *delegateFundsOptions) error {
		return err
	}
}
