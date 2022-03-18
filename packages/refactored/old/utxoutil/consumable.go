// Package utxoutil is used to build value transaction
package utxoutil

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// ConsumableOutput represents a wrapper of the output object. It allows 'consume' tokens and track
// how many remain to be consumed.
type ConsumableOutput struct {
	output      ledgerstate.Output
	remaining   map[ledgerstate.Color]uint64
	wasConsumed bool
}

// NewConsumables creates a slice of consumables out of slice of output objects.
func NewConsumables(out ...ledgerstate.Output) []*ConsumableOutput {
	ret := make([]*ConsumableOutput, len(out))
	for i, o := range out {
		ret[i] = NewConsumable(o)
	}
	return ret
}

// NewConsumable creates a consumable out of an output object.
func NewConsumable(output ledgerstate.Output) *ConsumableOutput {
	ret := &ConsumableOutput{
		output:    output,
		remaining: make(map[ledgerstate.Color]uint64),
	}
	output.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
		ret.remaining[col] = bal
		return true
	})
	return ret
}

// ToOutputs extracts output objects from consumables into the slice.
func ToOutputs(consumables ...*ConsumableOutput) []ledgerstate.Output {
	ret := make([]ledgerstate.Output, len(consumables))
	for i, c := range consumables {
		ret[i] = c.output
	}
	return ret
}

// Clone clones the consumable.
func (o *ConsumableOutput) Clone() *ConsumableOutput {
	ret := &ConsumableOutput{
		output:      o.output.Clone(),
		remaining:   make(map[ledgerstate.Color]uint64),
		wasConsumed: o.wasConsumed,
	}
	for col, bal := range o.remaining {
		ret.remaining[col] = bal
	}
	return ret
}

// ConsumableBalance return number of tokens remaining to consume on the consumable.
func (o *ConsumableOutput) ConsumableBalance(color ledgerstate.Color) uint64 {
	ret := o.remaining[color]
	return ret
}

// WasConsumed return true if consumable was 'touched', it some tokens were already consumed.
func (o *ConsumableOutput) WasConsumed() bool {
	return o.wasConsumed
}

// NothingRemains returns true if no tokens remain to be consumed.
func (o *ConsumableOutput) NothingRemains() bool {
	for _, bal := range o.remaining {
		if bal != 0 {
			return false
		}
	}
	return true
}

// ConsumableBalance returns how many tokens of the given color can be consumed from remaining.
func ConsumableBalance(color ledgerstate.Color, consumables ...*ConsumableOutput) uint64 {
	ret := uint64(0)
	for _, out := range consumables {
		ret += out.ConsumableBalance(color)
	}
	return ret
}

// EnoughBalance checks if it is enough tokens of the given color remains in the consumables.
func EnoughBalance(color ledgerstate.Color, amount uint64, consumables ...*ConsumableOutput) bool {
	consumable := ConsumableBalance(color, consumables...)
	return consumable >= amount
}

// EnoughBalances checks is it is enough remaining tokens to consume whole collection of balances.
func EnoughBalances(amounts map[ledgerstate.Color]uint64, consumables ...*ConsumableOutput) bool {
	for color, amount := range amounts {
		if !EnoughBalance(color, amount, consumables...) {
			return false
		}
	}
	return true
}

// ConsumeColored specified amount of colored tokens sequentially from specified ConsumableOutputs
// return nil if it was a success.
// In case of failure ConsumableOutputs remaining unchanged.
func ConsumeColored(color ledgerstate.Color, amount uint64, consumables ...*ConsumableOutput) bool {
	if !EnoughBalance(color, amount, consumables...) {
		return false
	}
	MustConsumeColored(color, amount, consumables...)
	return true
}

// MustConsumeColored same as as ConsumeColor only panics on unsuccessful consume.
func MustConsumeColored(color ledgerstate.Color, amount uint64, consumables ...*ConsumableOutput) {
	remaining := amount
	for _, out := range consumables {
		if remaining == 0 {
			break
		}
		rem := out.remaining[color]
		if rem == 0 {
			continue
		}
		if rem >= remaining {
			out.remaining[color] = rem - remaining
			remaining = 0
		} else {
			out.remaining[color] = 0
			remaining -= rem
		}
		out.wasConsumed = true
	}
	if remaining != 0 {
		panic("ConsumeColored: internal error")
	}
}

// ConsumeMany consumes whole collection of colored balances.
func ConsumeMany(amounts map[ledgerstate.Color]uint64, consumables ...*ConsumableOutput) bool {
	if !EnoughBalances(amounts, consumables...) {
		return false
	}
	for color, amount := range amounts {
		MustConsumeColored(color, amount, consumables...)
	}
	return true
}

// ConsumeRemaining consumes all remaining tokens and return map of wasConsumed balances.
func ConsumeRemaining(consumables ...*ConsumableOutput) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	for _, out := range consumables {
		for col, bal := range out.remaining {
			if bal == 0 {
				continue
			}
			ConsumeColored(col, bal, out)
			total := ret[col]
			ret[col] = total + bal
		}
	}
	return ret
}

// SelectConsumed filters out untouched consumables and returns those which were consumed.
func SelectConsumed(consumables ...*ConsumableOutput) []*ConsumableOutput {
	ret := make([]*ConsumableOutput, 0)
	for _, out := range consumables {
		if out.WasConsumed() {
			ret = append(ret, out)
		}
	}
	return ret
}

// MakeUTXOInputs from the list of consumables makes sorted inputs with NewInputs() and returns corresponding
// outputs in the same (changed) order.
func MakeUTXOInputs(consumables ...*ConsumableOutput) (ledgerstate.Inputs, []ledgerstate.Output) {
	inputs := make(ledgerstate.Inputs, len(consumables))
	origOrderOfInputs := make([]ledgerstate.OutputID, len(inputs))
	for i, out := range consumables {
		inp := ledgerstate.NewUTXOInput(out.output.ID())
		origOrderOfInputs[i] = inp.ReferencedOutputID()
		inputs[i] = inp
	}
	// Sorts!!! So we have to track corresponding outputs too
	retInputs := ledgerstate.NewInputs(inputs...)
	if len(retInputs) != len(origOrderOfInputs) {
		panic("duplicate inputs")
	}
	permutation := getPermutation(retInputs)
	retConsumedOutputs := make([]ledgerstate.Output, len(retInputs))
	for _, out := range consumables {
		retConsumedOutputs[permutation[out.output.ID()]] = out.output
	}
	return retInputs, retConsumedOutputs
}

// getPermutation utility function return mapping from outputID to its index in the slice.
func getPermutation(inputs ledgerstate.Inputs) map[ledgerstate.OutputID]int {
	ret := make(map[ledgerstate.OutputID]int)
	for i := range inputs {
		ret[inputs[i].(*ledgerstate.UTXOInput).ReferencedOutputID()] = i
	}
	return ret
}

// FindAliasConsumableInput finds chain output with given alias address.
func FindAliasConsumableInput(aliasAddr ledgerstate.Address, consumables ...*ConsumableOutput) (*ledgerstate.AliasOutput, int, bool) {
	for i, out := range consumables {
		if out.output.Address().Equals(aliasAddr) {
			if ret, ok := out.output.(*ledgerstate.AliasOutput); ok {
				return ret, i, true
			}
		}
	}
	return nil, 0, false
}
