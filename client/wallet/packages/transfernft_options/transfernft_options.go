package transfernft_options

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"golang.org/x/xerrors"
)

type TransferNFTOption func(options *transferNFTOptions) error

func WaitForConfirmation(wait bool) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which alias to transfer.
func Alias(aliasID string) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the new governor of the alias.
func ToAddress(address string) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// ResetStateAddress defines if the state address should be set to ToAddress as well, or not.
func ResetStateAddress(reset bool) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		options.ResetStateAddress = reset
		return nil
	}
}

// ResetDelegation defines if the output's delegation staus should be reset.
func ResetDelegation(reset bool) TransferNFTOption {
	return func(options *transferNFTOptions) error {
		options.ResetDelegation = reset
		return nil
	}
}

// transferNFTOptions is a struct that is used to aggregate the optional parameters in the TransferNFT call.
type transferNFTOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *ledgerstate.AliasAddress
	ToAddress             ledgerstate.Address
	WaitForConfirmation   bool
	ResetStateAddress     bool
	ResetDelegation       bool
}

func BuildTransferNFTOptions(options ...TransferNFTOption) (result *transferNFTOptions, err error) {
	// create options to collect the arguments provided
	result = &transferNFTOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, xerrors.Errorf("an alias identifier must be specified for transfer")
	}
	if result.ToAddress == nil {
		return nil, xerrors.Errorf("no receiving address specified for nft transfer")
	}

	return
}
