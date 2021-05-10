package createnft_options

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// CreateNFTOption is a function that provides options.
type CreateNFTOption func(options *createNFTOptions) error

// WaitForConfirmation defines if the call should wait for confirmation before it returns.
func WaitForConfirmation(wait bool) CreateNFTOption {
	return func(options *createNFTOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// InitialBalance sets the initial balance of the newly created NFT.
func InitialBalance(balance map[ledgerstate.Color]uint64) CreateNFTOption {
	return func(options *createNFTOptions) error {
		if balance[ledgerstate.ColorIOTA] < ledgerstate.DustThresholdAliasOutputIOTA {
			return xerrors.Errorf("NFT must have at least %d IOTA balance", ledgerstate.DustThresholdAliasOutputIOTA)
		}
		options.InitialBalance = balance
		return nil
	}
}

// ImmutableData sets the immutable data field of the freshly created NFT.
func ImmutableData(data []byte) CreateNFTOption {
	return func(options *createNFTOptions) error {
		if data == nil {
			return xerrors.Errorf("empty data supplied for immutable data")
		}
		if len(data) > ledgerstate.MaxOutputPayloadSize {
			return xerrors.Errorf("provided immutable data size %d is greater than maximum allowed %d", len(data), ledgerstate.MaxOutputPayloadSize)
		}
		options.ImmutableData = data
		return nil
	}
}

// AccessManaPledgeID is an option for SendFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) CreateNFTOption {
	return func(options *createNFTOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SendFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) CreateNFTOption {
	return func(options *createNFTOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// createNFTOptions is a struct that is used to aggregate the optional parameters in the CreateNFT call.
type createNFTOptions struct {
	InitialBalance        map[ledgerstate.Color]uint64
	ImmutableData         []byte
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// BuildCreateNFTOptions builds the options.
func BuildCreateNFTOptions(options ...CreateNFTOption) (result *createNFTOptions, err error) {
	// create options to collect the arguments provided
	result = &createNFTOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}
	if result.InitialBalance == nil {
		result.InitialBalance = map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: ledgerstate.DustThresholdAliasOutputIOTA}
	}

	return
}
