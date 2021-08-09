package sweepnftownednftsoptions

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// SweepNFTOwnedNFTsOption is a function that provides an option.
type SweepNFTOwnedNFTsOption func(options *SweepNFTOwnedNFTsOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) SweepNFTOwnedNFTsOption {
	return func(options *SweepNFTOwnedNFTsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) SweepNFTOwnedNFTsOption {
	return func(options *SweepNFTOwnedNFTsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) SweepNFTOwnedNFTsOption {
	return func(options *SweepNFTOwnedNFTsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which an aliasID that is checked for available funds.
func Alias(aliasID string) SweepNFTOwnedNFTsOption {
	return func(options *SweepNFTOwnedNFTsOptions) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the optional receiving address.
func ToAddress(address string) SweepNFTOwnedNFTsOption {
	return func(options *SweepNFTOwnedNFTsOptions) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// SweepNFTOwnedNFTsOptions is a struct that is used to aggregate the optional parameters in the sweepNFTOwnedNFTs call.
type SweepNFTOwnedNFTsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	ToAddress             ledgerstate.Address
	WaitForConfirmation   bool
}

// Build builds the options.
func Build(options ...SweepNFTOwnedNFTsOption) (result *SweepNFTOwnedNFTsOptions, err error) {
	// create options to collect the arguments provided
	result = &SweepNFTOwnedNFTsOptions{}

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
