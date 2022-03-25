package evilwallet

import (
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region Options ///////////////////////////////////////////////////////////////////////////

// Options is a struct that represents a collection of options that can be set when creating a message
type Options struct {
	aliasInputs              map[string]types.Empty
	inputs                   []ledgerstate.Output
	aliasOutputs             map[string]*ledgerstate.ColoredBalances
	outputs                  []*ledgerstate.ColoredBalances
	strongParents            map[string]types.Empty
	weakParents              map[string]types.Empty
	shallowLikeParents       map[string]types.Empty
	shallowDislikeParents    map[string]types.Empty
	inputWallet              *Wallet
	outputWallet             *Wallet
	issuingTime              time.Time
	reattachmentMessageAlias string
	sequenceNumber           uint64
	overrideSequenceNumber   bool
}

type OutputOption struct {
	aliasName string
	color     ledgerstate.Color
	amount    uint64
}

// NewOptions is the constructor for the MessageTestFrameworkMessageOptions.
func NewOptions(options ...Option) (messageOptions *Options) {
	messageOptions = &Options{
		aliasInputs:           make(map[string]types.Empty),
		inputs:                make([]ledgerstate.Output, 0),
		aliasOutputs:          make(map[string]*ledgerstate.ColoredBalances),
		outputs:               make([]*ledgerstate.ColoredBalances, 0),
		strongParents:         make(map[string]types.Empty),
		weakParents:           make(map[string]types.Empty),
		shallowLikeParents:    make(map[string]types.Empty),
		shallowDislikeParents: make(map[string]types.Empty),
	}

	for _, option := range options {
		option(messageOptions)
	}
	return
}

// Option is the type that is used for options that can be passed into the CreateMessage method to configure its
// behavior.
type Option func(*Options)

func (o *Options) isBalanceProvided() bool {
	provided := false

	for _, balance := range o.aliasOutputs {
		balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if balance > 0 {
				provided = true
			}
			return true
		})
	}
	return provided
}

func (o *Options) areInputsProvidedWithoutAliases() bool {
	return len(o.inputs) > 0
}

func (o *Options) areOutputsProvidedWithoutAliases() bool {
	return len(o.outputs) > 0
}

// checkInputsAndOutputs checks if either all provided inputs/outputs are with aliases or all are without,
// we do not allow for mixing those two possibilities.
func (o *Options) checkInputsAndOutputs() error {
	inLength, outLength, aliasInLength, aliasOutLength := len(o.inputs), len(o.outputs), len(o.aliasInputs), len(o.aliasOutputs)

	if (inLength == 0 && aliasInLength == 0) || (outLength == 0 && aliasOutLength == 0) {
		return errors.New("no inputs or outputs provided")
	}

	inputsOk := (inLength > 0 && aliasInLength == 0) || (aliasInLength > 0 && inLength == 0)
	outputsOk := (outLength > 0 && aliasOutLength == 0) || (aliasOutLength > 0 && outLength == 0)
	if !inputsOk || !outputsOk {
		return errors.New("mixing providing inputs/outputs with and without aliases is not allowed")
	}
	return nil
}

// WithInputs returns an Option that is used to provide the Inputs of the Transaction.
func WithInputs(inputs ...interface{}) Option {
	return func(options *Options) {
		for _, input := range inputs {
			switch in := input.(type) {
			case string:
				options.aliasInputs[in] = types.Void
			case ledgerstate.Output:
				options.inputs = append(options.inputs, in)
			}
		}
	}
}

// WithOutput returns an Option that is used to define a non-colored Output for the Transaction in the Message.
func WithOutput(output *OutputOption) Option {
	return func(options *Options) {
		if output.aliasName != "" {
			options.aliasOutputs[output.aliasName] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				output.color: output.amount,
			})
			return
		} else {
			options.outputs = append(options.outputs, ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				output.color: output.amount,
			}))
		}
	}
}

// WithOutputs returns an Option that is used to define a non-colored Outputs for the Transaction in the Message.
func WithOutputs(outputs []*OutputOption) Option {
	return func(options *Options) {
		for _, output := range outputs {
			if output.aliasName != "" {
				options.aliasOutputs[output.aliasName] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
					output.color: output.amount,
				})
			} else {
				options.outputs = append(options.outputs, ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
					output.color: output.amount,
				}))
			}
		}
	}
}

// WithStrongParents returns an Option that is used to define the strong parents of the Message.
func WithStrongParents(messageAliases ...string) Option {
	return func(options *Options) {
		for _, messageAlias := range messageAliases {
			options.strongParents[messageAlias] = types.Void
		}
	}
}

// WithWeakParents returns an Option that is used to define the weak parents of the Message.
func WithWeakParents(messageAliases ...string) Option {
	return func(options *Options) {
		for _, messageAlias := range messageAliases {
			options.weakParents[messageAlias] = types.Void
		}
	}
}

// WithShallowLikeParents returns a MessageOption that is used to define the shallow like parents of the Message.
func WithShallowLikeParents(messageAliases ...string) Option {
	return func(options *Options) {
		for _, messageAlias := range messageAliases {
			options.shallowLikeParents[messageAlias] = types.Void
		}
	}
}

// WithShallowDislikeParents returns a MessageOption that is used to define the shallow dislike parents of the Message.
func WithShallowDislikeParents(messageAliases ...string) Option {
	return func(options *Options) {
		for _, messageAlias := range messageAliases {
			options.shallowDislikeParents[messageAlias] = types.Void
		}
	}
}

// WithIssuer returns a MessageOption that is used to define the inputWallet of the Message.
func WithIssuer(issuer *Wallet) Option {
	return func(options *Options) {
		options.inputWallet = issuer
	}
}

// WithOutputWallet returns a MessageOption that is used to define the inputWallet of the Message.
func WithOutputWallet(wallet *Wallet) Option {
	return func(options *Options) {
		options.outputWallet = wallet
	}
}

// WithIssuingTime returns a MessageOption that is used to set issuing time of the Message.
func WithIssuingTime(issuingTime time.Time) Option {
	return func(options *Options) {
		options.issuingTime = issuingTime
	}
}

// ConflictSlice represents a set of conflict transactions.
type ConflictSlice [][]Option

// endregion  //////////////////////////////////////////////////////////////////////////////////////////////////////////

// region FaucetRequestOptions /////////////////////////////////////////////////////////////////////////////////////////

// FaucetRequestOptions is options for faucet request.
type FaucetRequestOptions struct {
	outputAliasName string
}

// NewFaucetRequestOptions creates options for a faucet request.
func NewFaucetRequestOptions(options ...FaucetRequestOption) *FaucetRequestOptions {
	reqOptions := &FaucetRequestOptions{
		outputAliasName: "",
	}

	for _, option := range options {
		option(reqOptions)
	}

	return reqOptions
}

// FaucetRequestOption is an option for faucet request.
type FaucetRequestOption func(*FaucetRequestOptions)

// WithOutputAlias returns an Option that is used to provide the Output of the Transaction.
func WithOutputAlias(aliasName string) FaucetRequestOption {
	return func(options *FaucetRequestOptions) {
		options.outputAliasName = aliasName
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
