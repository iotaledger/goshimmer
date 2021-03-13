// package to build value transaction
package utxoutil

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type ConsumableOutput struct {
	output      ledgerstate.Output
	remaining   map[ledgerstate.Color]uint64
	wasConsumed bool
}

func NewConsumables(out ...ledgerstate.Output) []*ConsumableOutput {
	ret := make([]*ConsumableOutput, len(out))
	for i, o := range out {
		ret[i] = &ConsumableOutput{
			output:    o,
			remaining: make(map[ledgerstate.Color]uint64),
		}
		o.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
			ret[i].remaining[col] = bal
			return true
		})
	}
	return ret
}

func ToOutputs(consumables ...*ConsumableOutput) []ledgerstate.Output {
	ret := make([]ledgerstate.Output, len(consumables))
	for i, c := range consumables {
		ret[i] = c.output
	}
	return ret
}

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

func (o *ConsumableOutput) ConsumableBalance(color ledgerstate.Color) uint64 {
	ret, _ := o.remaining[color]
	return ret
}

func (o *ConsumableOutput) WasConsumed() bool {
	return o.wasConsumed
}

func (o *ConsumableOutput) NothingRemains() bool {
	for _, bal := range o.remaining {
		if bal != 0 {
			return false
		}
	}
	return true
}

func ConsumableBalance(color ledgerstate.Color, consumables ...*ConsumableOutput) uint64 {
	ret := uint64(0)
	for _, out := range consumables {
		ret += out.ConsumableBalance(color)
	}
	return ret
}

func EnoughBalance(color ledgerstate.Color, amount uint64, consumables ...*ConsumableOutput) bool {
	consumable := ConsumableBalance(color, consumables...)
	return consumable >= amount
}

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
// In case of failure ConsumableOutputs remaining unchanged
func ConsumeColored(color ledgerstate.Color, amount uint64, consumables ...*ConsumableOutput) bool {
	if !EnoughBalance(color, amount, consumables...) {
		return false
	}
	MustConsumeColored(color, amount, consumables...)
	return true
}

func MustConsumeColored(color ledgerstate.Color, amount uint64, consumables ...*ConsumableOutput) {
	remaining := amount
	for _, out := range consumables {
		if remaining == 0 {
			break
		}
		rem, _ := out.remaining[color]
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

func ConsumeMany(amounts map[ledgerstate.Color]uint64, consumables ...*ConsumableOutput) bool {
	if !EnoughBalances(amounts, consumables...) {
		return false
	}
	for color, amount := range amounts {
		MustConsumeColored(color, amount, consumables...)
	}
	return true
}

// ConsumeRemaining consumes all remaining tokens and return map of wasConsumed balances
func ConsumeRemaining(consumables ...*ConsumableOutput) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	for _, out := range consumables {
		for col, bal := range out.remaining {
			if bal == 0 {
				continue
			}
			ConsumeColored(col, bal, out)
			total, _ := ret[col]
			ret[col] = total + bal
		}
	}
	return ret
}

func SelectConsumed(consumables ...*ConsumableOutput) []*ConsumableOutput {
	ret := make([]*ConsumableOutput, 0)
	for _, out := range consumables {
		if out.WasConsumed() {
			ret = append(ret, out)
		}
	}
	return ret
}

func MakeUTXOInputs(consumables ...*ConsumableOutput) (ledgerstate.Inputs, []ledgerstate.Output) {
	retInputs := make(ledgerstate.Inputs, len(consumables))
	retConsumedOutputs := make([]ledgerstate.Output, len(consumables))
	for i, out := range consumables {
		retInputs[i] = ledgerstate.NewUTXOInput(out.output.ID())
		retConsumedOutputs[i] = out.output
	}
	return retInputs, retConsumedOutputs
}

func FindChainConsumableInput(aliasAddr ledgerstate.Address, consumables ...*ConsumableOutput) (*ledgerstate.ChainOutput, int, bool) {
	for i, out := range consumables {
		if EqualAddresses(out.output.Address(), aliasAddr) {
			if ret, ok := out.output.(*ledgerstate.ChainOutput); ok {
				return ret, i, true
			}
		}
	}
	return nil, 0, false
}
