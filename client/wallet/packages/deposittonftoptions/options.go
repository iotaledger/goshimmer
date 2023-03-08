package deposittonftoptions

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// DepositFundsToNFTOption is a function that provides options.
type DepositFundsToNFTOption func(options *DepositFundsToNFTOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) DepositFundsToNFTOption {
	return func(options *DepositFundsToNFTOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// Amount sets how much funds should be withdrew.
func Amount(amount map[devnetvm.Color]uint64) DepositFundsToNFTOption {
	return func(options *DepositFundsToNFTOptions) error {
		options.Amount = amount
		return nil
	}
}

// Alias specifies which alias to transfer.
func Alias(aliasID string) DepositFundsToNFTOption {
	return func(options *DepositFundsToNFTOptions) error {
		parsed, err := devnetvm.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) DepositFundsToNFTOption {
	return func(options *DepositFundsToNFTOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) DepositFundsToNFTOption {
	return func(options *DepositFundsToNFTOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// DepositFundsToNFTOptions is a struct that is used to aggregate the optional parameters in the DepositFundsToNFT call.
type DepositFundsToNFTOptions struct {
	Amount                map[devnetvm.Color]uint64
	Alias                 *devnetvm.AliasAddress
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// Build build the options.
func Build(options ...DepositFundsToNFTOption) (result *DepositFundsToNFTOptions, err error) {
	// create options to collect the arguments provided
	result = &DepositFundsToNFTOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}
	if result.Alias == nil {
		return nil, errors.New("an alias identifier must be specified for withdrawal")
	}

	if result.Amount == nil {
		return nil, errors.New("no funds provided for deposit")
	}

	return
}
