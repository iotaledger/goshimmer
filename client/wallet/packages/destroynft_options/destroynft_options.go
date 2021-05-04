package destroynft_options

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"golang.org/x/xerrors"
)

type DestroyNFTOption func(options *destroyNFTOption) error

func WaitForConfirmation(wait bool) DestroyNFTOption {
	return func(options *destroyNFTOption) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for DestroyNFT call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) DestroyNFTOption {
	return func(options *destroyNFTOption) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for DestroyNFT call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) DestroyNFTOption {
	return func(options *destroyNFTOption) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which alias to destroy.
func Alias(aliasID string) DestroyNFTOption {
	return func(options *destroyNFTOption) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// RemainderAddress specifies the address where the funds of the destroyed NFT will be sent. (optional)
func RemainderAddress(address string) DestroyNFTOption {
	return func(options *destroyNFTOption) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.RemainderAddress = parsed
		return nil
	}
}

// destroyNFTOption is a struct that is used to aggregate the optional parameters in the DestroyNFT call.
type destroyNFTOption struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	RemainderAddress      ledgerstate.Address
	WaitForConfirmation   bool
}

func BuildDestroyNFTOptions(options ...DestroyNFTOption) (result *destroyNFTOption, err error) {
	// create options to collect the arguments provided
	result = &destroyNFTOption{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, xerrors.Errorf("an alias identifier must be specified for destroy")
	}

	return
}
