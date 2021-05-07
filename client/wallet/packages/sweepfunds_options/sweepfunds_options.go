package sweepfunds_options

type SweepFundsOption func(options *sweepFundsOptions) error

func WaitForConfirmation(wait bool) SweepFundsOption {
	return func(options *sweepFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) SweepFundsOption {
	return func(options *sweepFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) SweepFundsOption {
	return func(options *sweepFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// sweepNFTOwnedNFTsOptions is a struct that is used to aggregate the optional parameters in the sweepFunds call.
type sweepFundsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

func BuildSweepFundsOptions(options ...SweepFundsOption) (result *sweepFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &sweepFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	return
}
