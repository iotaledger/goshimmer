package consolidateoptions

// ConsolidateFundsOption is a function that provides options.
type ConsolidateFundsOption func(options *ConsolidateFundsOptions) error

// WaitForConfirmation is an optional parameter to define if the consolidateFunds command should wait for confirmation
// before it returns.
func WaitForConfirmation(wait bool) ConsolidateFundsOption {
	return func(options *ConsolidateFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) ConsolidateFundsOption {
	return func(options *ConsolidateFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for SweepNFTOwnedFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) ConsolidateFundsOption {
	return func(options *ConsolidateFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// ConsolidateFundsOptions is a struct that is used to aggregate the optional parameters in the consolidateFunds call.
type ConsolidateFundsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// Build build the options.
func Build(options ...ConsolidateFundsOption) (result *ConsolidateFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &ConsolidateFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	return
}
