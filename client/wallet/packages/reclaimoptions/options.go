package reclaimoptions

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// ReclaimFundsOption is a function that provides an option.
type ReclaimFundsOption func(options *reclaimFundsOption) error

func WaitForConfirmation(wait bool) ReclaimFundsOption {
	return func(options *reclaimFundsOption) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) ReclaimFundsOption {
	return func(options *reclaimFundsOption) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) ReclaimFundsOption {
	return func(options *reclaimFundsOption) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which alias to reclaim.
func Alias(aliasID string) ReclaimFundsOption {
	return func(options *reclaimFundsOption) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the new governor of the alias.
func ToAddress(address string) ReclaimFundsOption {
	return func(options *reclaimFundsOption) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// reclaimFundsOption is a struct that is used to aggregate the optional parameters in the ReclaimDelegatedFunds call.
type reclaimFundsOption struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	ToAddress             ledgerstate.Address
	WaitForConfirmation   bool
}

// Build build the options.
func Build(options ...ReclaimFundsOption) (result *reclaimFundsOption, err error) {
	// create options to collect the arguments provided
	result = &reclaimFundsOption{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, xerrors.Errorf("an alias identifier must be specified for reclaiming delegated funds")
	}

	return
}
