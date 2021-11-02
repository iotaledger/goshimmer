package destroynftoptions

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// DestroyNFTOption is a function that provides options.
type DestroyNFTOption func(options *DestroyNFTOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) DestroyNFTOption {
	return func(options *DestroyNFTOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for DestroyNFT call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) DestroyNFTOption {
	return func(options *DestroyNFTOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for DestroyNFT call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) DestroyNFTOption {
	return func(options *DestroyNFTOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which alias to destroy.
func Alias(aliasID string) DestroyNFTOption {
	return func(options *DestroyNFTOptions) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// RemainderAddress specifies the address where the funds of the destroyed NFT will be sent (optional).
func RemainderAddress(address string) DestroyNFTOption {
	return func(options *DestroyNFTOptions) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.RemainderAddress = parsed
		return nil
	}
}

// DestroyNFTOptions is a struct that is used to aggregate the optional parameters in the DestroyNFT call.
type DestroyNFTOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	RemainderAddress      ledgerstate.Address
	WaitForConfirmation   bool
}

// Build builds the options.
func Build(options ...DestroyNFTOption) (result *DestroyNFTOptions, err error) {
	// create options to collect the arguments provided
	result = &DestroyNFTOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, errors.Errorf("an alias identifier must be specified for destroy")
	}

	return
}
