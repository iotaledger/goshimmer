package evilwallet

import (
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region Options ///////////////////////////////////////////////////////////////////////////

// Options is a struct that represents a collection of options that can be set when creating a message
type Options struct {
	inputs                   map[string]types.Empty
	outputs                  map[string]*ledgerstate.ColoredBalances
	strongParents            map[string]types.Empty
	weakParents              map[string]types.Empty
	shallowLikeParents       map[string]types.Empty
	shallowDislikeParents    map[string]types.Empty
	issuer                   *Wallet
	outputWallet             *Wallet
	issuingTime              time.Time
	reattachmentMessageAlias string
	sequenceNumber           uint64
	overrideSequenceNumber   bool
}

// NewOptions is the constructor for the MessageTestFrameworkMessageOptions.
func NewOptions(options ...Option) (messageOptions *Options) {
	messageOptions = &Options{
		inputs:                make(map[string]types.Empty),
		outputs:               make(map[string]*ledgerstate.ColoredBalances),
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
	for _, balance := range o.outputs {
		balance.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if balance > 0 {
				provided = true
			}
			return true
		})
	}
	return provided
}

// WithInputs returns an Option that is used to provide the Inputs of the Transaction.
func WithInputs(inputAliases ...string) Option {
	return func(options *Options) {
		for _, inputAlias := range inputAliases {
			options.inputs[inputAlias] = types.Void
		}
	}
}

// WithOutput returns an Option that is used to define a non-colored Output for the Transaction in the Message.
func WithOutput(outputAlias string, balance uint64) Option {
	return func(options *Options) {
		options.outputs[outputAlias] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: balance,
		})
	}
}

// WithOutputs returns an Option that is used to define a non-colored Outputs for the Transaction in the Message.
func WithOutputs(outputAliases []string, balances ...uint64) Option {
	return func(options *Options) {
		for i, inputAlias := range outputAliases {
			if len(balances) > 0 {
				options.outputs[inputAlias] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: balances[i],
				})
			} else {
				options.outputs[inputAlias] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: 0,
				})
			}
		}
	}
}

// WithColoredOutput returns an Option that is used to define a colored Output for the Transaction in the Message.
func WithColoredOutput(outputAlias string, color ledgerstate.Color, amount uint64) Option {
	return func(options *Options) {
		balance := ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			color: amount,
		})
		options.outputs[outputAlias] = balance
	}
}

// WithColoredOutputs returns an Option that is used to define a colored Outputs for the Transaction in the Message.
func WithColoredOutputs(outputAliases []string, colors []ledgerstate.Color, amount []uint64) Option {
	return func(options *Options) {
		for i, inputAlias := range outputAliases {
			balance := ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				colors[i]: amount[i],
			})
			options.outputs[inputAlias] = balance
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

// WithIssuer returns a MessageOption that is used to define the issuer of the Message.
func WithIssuer(issuer *Wallet) Option {
	return func(options *Options) {
		options.issuer = issuer
	}
}

// WithOutputWallet returns a MessageOption that is used to define the issuer of the Message.
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

// region ConflictMap ///////////////////////////////////////////////////////////////////////////

// ConflictMap represents a set of conflict transactions.
type ConflictMap map[string][]Option

// region FaucetRequestOptions ///////////////////////////////////////////////////////////////////////////

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
