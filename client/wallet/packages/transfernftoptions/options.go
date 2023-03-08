package transfernftoptions

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// TransferNFTOption is a function that provides an option.
type TransferNFTOption func(options *TransferNFTOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// Alias specifies which alias to transfer.
func Alias(aliasID string) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		parsed, err := devnetvm.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the new governor of the alias.
func ToAddress(address string) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		parsed, err := devnetvm.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// ResetStateAddress defines if the state address should be set to ToAddress as well, or not.
func ResetStateAddress(reset bool) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		options.ResetStateAddress = reset
		return nil
	}
}

// ResetDelegation defines if the output's delegation staus should be reset.
func ResetDelegation(reset bool) TransferNFTOption {
	return func(options *TransferNFTOptions) error {
		options.ResetDelegation = reset
		return nil
	}
}

// TransferNFTOptions is a struct that is used to aggregate the optional parameters in the TransferNFT call.
type TransferNFTOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	Alias                 *devnetvm.AliasAddress
	ToAddress             devnetvm.Address
	WaitForConfirmation   bool
	ResetStateAddress     bool
	ResetDelegation       bool
}

// Build build the options.
func Build(options ...TransferNFTOption) (result *TransferNFTOptions, err error) {
	// create options to collect the arguments provided
	result = &TransferNFTOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	if result.Alias == nil {
		return nil, errors.New("an alias identifier must be specified for transfer")
	}
	if result.ToAddress == nil {
		return nil, errors.New("no receiving address specified for nft transfer")
	}

	return
}
