package withdrawfundsfromnft_options

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"golang.org/x/xerrors"
)

type WithdrawFundsFromNFTOption func(options *withdrawFundsFromNFTOption) error

// WaitForConfirmation
func WaitForConfirmation(wait bool) WithdrawFundsFromNFTOption {
	return func(options *withdrawFundsFromNFTOption) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// Amount sets how much funds should be withdrew.
func Amount(amount map[ledgerstate.Color]uint64) WithdrawFundsFromNFTOption {
	return func(options *withdrawFundsFromNFTOption) error {
		options.Amount = amount
		return nil
	}
}

// Alias specifies which alias to transfer.
func Alias(aliasID string) WithdrawFundsFromNFTOption {
	return func(options *withdrawFundsFromNFTOption) error {
		parsed, err := ledgerstate.AliasAddressFromBase58EncodedString(aliasID)
		if err != nil {
			return err
		}
		options.Alias = parsed
		return nil
	}
}

// ToAddress specifies the new governor of the alias.
func ToAddress(address string) WithdrawFundsFromNFTOption {
	return func(options *withdrawFundsFromNFTOption) error {
		parsed, err := ledgerstate.AddressFromBase58EncodedString(address)
		if err != nil {
			return err
		}
		options.ToAddress = parsed
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) WithdrawFundsFromNFTOption {
	return func(options *withdrawFundsFromNFTOption) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) WithdrawFundsFromNFTOption {
	return func(options *withdrawFundsFromNFTOption) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// withdrawFundsFromNFTOption is a struct that is used to aggregate the optional parameters in the CreateNFT call.
type withdrawFundsFromNFTOption struct {
	Amount                map[ledgerstate.Color]uint64
	Alias                 *ledgerstate.AliasAddress
	ToAddress             ledgerstate.Address
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

func BuildWithdrawFundsFromNFTOptions(options ...WithdrawFundsFromNFTOption) (result *withdrawFundsFromNFTOption, err error) {
	// create options to collect the arguments provided
	result = &withdrawFundsFromNFTOption{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}
	if result.Alias == nil {
		return nil, xerrors.Errorf("an alias identifier must be specified for withdrawal")
	}

	return
}
