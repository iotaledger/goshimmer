package sendoptions

import (
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/constants"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// SendFundsOption is the type for the optional parameters for the SendFunds call.
type SendFundsOption func(*SendFundsOptions) error

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
	return func(options *SendFundsOptions) error {
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
	return func(options *SendFundsOptions) error {
		options.RemainderAddress = addr

		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) SendFundsOption {
	return func(options *SendFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) SendFundsOption {
	return func(options *SendFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) SendFundsOption {
	return func(options *SendFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// UsePendingOutputs defines if we can collect outputs that are still pending confirmation.
func UsePendingOutputs(usePendingOutputs bool) SendFundsOption {
	return func(options *SendFundsOptions) error {
		options.UsePendingOutputs = usePendingOutputs
		return nil
	}
}

// LockUntil is an option for SendFunds call that defines if the created outputs should be locked until a certain time.
func LockUntil(until time.Time) SendFundsOption {
	return func(options *SendFundsOptions) error {
		if until.Before(time.Now()) {
			return errors.Errorf("can't timelock funds in the past")
		}
		if until.After(constants.MaxRepresentableTime) {
			return errors.Errorf("invalid timelock: %s is later, than max representable time %s",
				until.String(), constants.MaxRepresentableTime.String())
		}
		options.LockUntil = until
		return nil
	}
}

// Fallback defines the parameters for conditional sending: fallback address and fallback deadline.
// If the output is not spent by the recipient within the fallback deadline, only fallback address is able to unlock it.
func Fallback(addy ledgerstate.Address, deadline time.Time) SendFundsOption {
	return func(options *SendFundsOptions) error {
		if addy == nil {
			return errors.Errorf("empty fallback address provided")
		}
		if deadline.Before(time.Now()) {
			return errors.Errorf("invalid fallback deadline: %s is in the past", deadline.String())
		}
		if deadline.After(constants.MaxRepresentableTime) {
			return errors.Errorf("invalid fallback deadline: %s is later, than max representable time %s",
				deadline.String(), constants.MaxRepresentableTime.String())
		}
		options.FallbackAddress = addy
		options.FallbackDeadline = deadline
		return nil
	}
}

// SendFundsOptions is a struct that is used to aggregate the optional parameters provided in the SendFunds call.
type SendFundsOptions struct {
	Destinations          map[address.Address]map[ledgerstate.Color]uint64
	RemainderAddress      address.Address
	LockUntil             time.Time
	FallbackAddress       ledgerstate.Address
	FallbackDeadline      time.Time
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
	UsePendingOutputs     bool
}

// RequiredFunds derives how much funds are needed based on the Destinations to fund the transfer.
func (s *SendFundsOptions) RequiredFunds() map[ledgerstate.Color]uint64 {
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

// Build is a utility function that constructs the SendFundsOptions.
func Build(options ...SendFundsOption) (result *SendFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &SendFundsOptions{}

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
	return func(options *SendFundsOptions) error {
		return err
	}
}
