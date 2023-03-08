package reclaimoptions

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// ReclaimFundsOption is a function that provides an option.
type ReclaimFundsOption func(options *ReclaimFundsOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) ReclaimFundsOption {
	return func(options *ReclaimFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) ReclaimFundsOption {
	return func(options *ReclaimFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) ReclaimFundsOption {
	return func(options *ReclaimFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which alias to reclaim.
func Alias(aliasID string) ReclaimFundsOption {
	return func(options *ReclaimFundsOptions) error {
		parsed, err := devnetvm.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the new governor of the alias.
func ToAddress(address string) ReclaimFundsOption {
	return func(options *ReclaimFundsOptions) error {
		parsed, err := devnetvm.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// ReclaimFundsOptions is a struct that is used to aggregate the optional parameters in the ReclaimDelegatedFunds call.
type ReclaimFundsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *devnetvm.AliasAddress
	ToAddress             devnetvm.Address
	WaitForConfirmation   bool
}

// Build build the options.
func Build(options ...ReclaimFundsOption) (result *ReclaimFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &ReclaimFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, errors.New("an alias identifier must be specified for reclaiming delegated funds")
	}

	return
}
