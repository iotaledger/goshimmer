package depositfundstonft_options

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// DepositFundsToNFTOption is a function that provides options.
type DepositFundsToNFTOption func(options *depositFundsToNFTOption) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) DepositFundsToNFTOption {
	return func(options *depositFundsToNFTOption) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// Amount sets how much funds should be withdrew.
func Amount(amount map[ledgerstate.Color]uint64) DepositFundsToNFTOption {
	return func(options *depositFundsToNFTOption) error {
		options.Amount = amount
		return nil
	}
}

// Alias specifies which alias to transfer.
func Alias(aliasID string) DepositFundsToNFTOption {
	return func(options *depositFundsToNFTOption) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) DepositFundsToNFTOption {
	return func(options *depositFundsToNFTOption) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) DepositFundsToNFTOption {
	return func(options *depositFundsToNFTOption) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// depositFundsToNFTOption is a struct that is used to aggregate the optional parameters in the DepositFundsToNFT call.
type depositFundsToNFTOption struct {
	Amount                map[ledgerstate.Color]uint64
	Alias                 *ledgerstate.AliasAddress
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// BuildDepositFundsToNFTOptions build the options.
func BuildDepositFundsToNFTOptions(options ...DepositFundsToNFTOption) (result *depositFundsToNFTOption, err error) {
	// create options to collect the arguments provided
	result = &depositFundsToNFTOption{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}
	if result.Alias == nil {
		return nil, xerrors.Errorf("an alias identifier must be specified for withdrawal")
	}

	if result.Amount == nil {
		return nil, xerrors.Errorf("no funds provided for deposit")
	}

	return
}
