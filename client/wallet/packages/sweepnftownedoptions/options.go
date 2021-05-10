package sweepnftownedoptions

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// SweepNFTOwnedFundsOption is a function that provides option.
type SweepNFTOwnedFundsOption func(options *sweepNFTOwnedFundsOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) SweepNFTOwnedFundsOption {
	return func(options *sweepNFTOwnedFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) SweepNFTOwnedFundsOption {
	return func(options *sweepNFTOwnedFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) SweepNFTOwnedFundsOption {
	return func(options *sweepNFTOwnedFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which an aliasID that is checked for available funds.
func Alias(aliasID string) SweepNFTOwnedFundsOption {
	return func(options *sweepNFTOwnedFundsOptions) error {
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
	return func(options *sweepNFTOwnedFundsOptions) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// sweepNFTOwnedFundsOptions is a struct that is used to aggregate the optional parameters in the SweepNFTOwnedFunds call.
type sweepNFTOwnedFundsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	ToAddress             ledgerstate.Address
	WaitForConfirmation   bool
}

// Build build the options.
func Build(options ...SweepNFTOwnedFundsOption) (result *sweepNFTOwnedFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &sweepNFTOwnedFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, xerrors.Errorf("an nft identifier must be specified to sweep funds from")
	}

	return
}
