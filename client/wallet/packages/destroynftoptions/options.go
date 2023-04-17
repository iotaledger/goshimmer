package destroynftoptions

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
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
		parsed, err := devnetvm.AliasAddressFromBase58EncodedString(aliasID)
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
		parsed, err := devnetvm.AddressFromBase58EncodedString(address)
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
	Alias                 *devnetvm.AliasAddress
	RemainderAddress      devnetvm.Address
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
		return nil, errors.New("an alias identifier must be specified for destroy")
	}

	return
}
