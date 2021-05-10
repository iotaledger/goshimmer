package claimconditionaloptions

// ClaimConditionalFundsOption is a function that provides options.
type ClaimConditionalFundsOption func(options *claimConditionalFundsOptions) error

// WaitForConfirmation is an optional parameter to define if the ClaimConditionalFunds command should wait for confirmation
// before it returns.
func WaitForConfirmation(wait bool) ClaimConditionalFundsOption {
	return func(options *claimConditionalFundsOptions) error {
		options.WaitForConfirmation = wait
		return nil
	}
}

// AccessManaPledgeID is an option for ClaimConditionalFunds call that defines the nodeID to pledge access mana to.
func AccessManaPledgeID(nodeID string) ClaimConditionalFundsOption {
	return func(options *claimConditionalFundsOptions) error {
		options.AccessManaPledgeID = nodeID
		return nil
	}
}

// ConsensusManaPledgeID is an option for ClaimConditionalFunds call that defines the nodeID to pledge consensus mana to.
func ConsensusManaPledgeID(nodeID string) ClaimConditionalFundsOption {
	return func(options *claimConditionalFundsOptions) error {
		options.ConsensusManaPledgeID = nodeID
		return nil
	}
}

// claimConditionalFundsOptions is a struct that is used to aggregate the optional parameters in the claimConditionalFunds call.
type claimConditionalFundsOptions struct {
	AccessManaPledgeID    string
	ConsensusManaPledgeID string
	WaitForConfirmation   bool
}

// Build builds the options.
func Build(options ...ClaimConditionalFundsOption) (result *claimConditionalFundsOptions, err error) {
	// create options to collect the arguments provided
	result = &claimConditionalFundsOptions{}

	// apply arguments to our options
	for _, option := range options {
		if err = option(result); err != nil {
			return
		}
	}

	return
}
