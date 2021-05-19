package sweepnftownedoptions

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// SweepNFTOwnedFundsOption is a function that provides option.
type SweepNFTOwnedFundsOption func(options *SweepNFTOwnedFundsOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) SweepNFTOwnedFundsOption {
	return func(options *SweepNFTOwnedFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) SweepNFTOwnedFundsOption {
	return func(options *SweepNFTOwnedFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) SweepNFTOwnedFundsOption {
	return func(options *SweepNFTOwnedFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which an aliasID that is checked for available funds.
func Alias(aliasID string) SweepNFTOwnedFundsOption {
	return func(options *SweepNFTOwnedFundsOptions) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the optional receiving address.
func ToAddress(address string) SweepNFTOwnedFundsOption {
	return func(options *SweepNFTOwnedFundsOptions) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// SweepNFTOwnedFundsOptions is a struct that is used to aggregate the optional parameters in the SweepNFTOwnedFunds call.
type SweepNFTOwnedFundsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	ToAddress             ledgerstate.Address
	WaitForConfirmation   bool
}

// Build build the options.
func Build(options ...SweepNFTOwnedFundsOption) (result *SweepNFTOwnedFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &SweepNFTOwnedFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, errors.Errorf("an nft identifier must be specified to sweep funds from")
	}

	return
}
