package wallet

import (
	"errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// SendFundsOption is the type for the optional parameters for the SendFunds call.
type SendFundsOption func(*sendFundsOptions) error

// Destination is an option for the SendFunds call that defines a destination for funds that are supposed to be moved.
func Destination(addr address.Address, amount uint64, optionalColor ...ledgerstate.Color) SendFundsOption {
	// determine optional output color
	var outputColor ledgerstate.Color
	switch len(optionalColor) {
	case 0:
		outputColor = ledgerstate.ColorIOTA
	case 1:
		outputColor = optionalColor[0]
	default:
		return optionError(errors.New("providing more than one output color for the destination of funds is forbidden"))
	}

	// return an error if the amount is less
	if amount == 0 {
		return optionError(errors.New("the amount provided in the destinations needs to be larger than 0"))
	}

	// return Option
	return func(options *sendFundsOptions) error {
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

// Remainder is an option for the SendsFunds call that allows us to specify the remainder address that is
// supposed to be used in the corresponding transaction.
func Remainder(addr address.Address) SendFundsOption {
	return func(options *sendFundsOptions) error {
		options.RemainderAddress = addr

		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) SendFundsOption {
	return func(options *sendFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) SendFundsOption {
	return func(options *sendFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// sendFundsOptions is a struct that is used to aggregate the optional parameters provided in the SendFunds call.
type sendFundsOptions struct {
	Destinations          map[address.Address]map[ledgerstate.Color]uint64
	RemainderAddress      address.Address
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
}

// buildSendFundsOptions is a utility function that constructs the sendFundsOptions.
func buildSendFundsOptions(options ...SendFundsOption) (result *sendFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &sendFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	// sanitize parameters
	if len(result.Destinations) == 0 {
		err = errors.New("you need to provide at least one Destination for a valid transfer to be issued")

		return
	}

	return
}

// optionError is a utility function that returns a Option that returns the error provided in the
// argument.
func optionError(err error) SendFundsOption {
	return func(options *sendFundsOptions) error {
		return err
	}
}
