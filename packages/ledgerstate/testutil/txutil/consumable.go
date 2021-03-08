// package to build value transaction
package txutil

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type ConsumableOutput struct {
	output   ledgerstate.Output
	remain   map[ledgerstate.Color]uint64
	consumed map[ledgerstate.Color]uint64
}

func NewConsumableOutput(out ledgerstate.Output) *ConsumableOutput {
	ret := &ConsumableOutput{
		output:   out,
		remain:   make(map[ledgerstate.Color]uint64),
		consumed: make(map[ledgerstate.Color]uint64),
	}
	out.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
		ret.remain[col] = bal
		return true
	})
	return ret
}

func (o *ConsumableOutput) Clone() *ConsumableOutput {
	ret := &ConsumableOutput{
		output:   o.output.Clone(),
		remain:   make(map[ledgerstate.Color]uint64),
		consumed: make(map[ledgerstate.Color]uint64),
	}
	for col, bal := range o.remain {
		ret.remain[col] = bal
	}
	for col, bal := range o.consumed {
		ret.consumed[col] = bal
	}
	return ret
}

func (o *ConsumableOutput) ConsumableBalance(color ledgerstate.Color) uint64 {
	ret, _ := o.remain[color]
	return ret
}

func (o *ConsumableOutput) WasConsumed() bool {
	return len(o.consumed) > 0
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
// In case of failure ConsumableOutputs remain unchanged
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
		rem, _ := out.remain[color]
		if rem == 0 {
			continue
		}
		cons, _ := out.consumed[color]
		if rem >= remaining {
			out.remain[color] = rem - remaining
			remaining = 0
			out.consumed[color] = cons + remaining
		} else {
			out.remain[color] = 0
			remaining -= rem
			out.consumed[color] = cons + rem
		}
	}
	if remaining != 0 {
		panic("ConsumeColored: internal error")
	}
}

func ConsumeIOTA(amount uint64, consumables ...*ConsumableOutput) bool {
	return ConsumeColored(ledgerstate.ColorIOTA, amount, consumables...)
}

func ConsumeAll(amounts map[ledgerstate.Color]uint64, consumables ...*ConsumableOutput) bool {
	if !EnoughBalances(amounts, consumables...) {
		return false
	}
	for color, amount := range amounts {
		MustConsumeColored(color, amount, consumables...)
	}
	return true
}

// ConsumeRemaining consumes all remaining tokens and return map of consumed balances
func ConsumeRemaining(consumables ...*ConsumableOutput) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	for _, out := range consumables {
		for col, bal := range out.remain {
			if bal == 0 {
				continue
			}
			cons, _ := out.consumed[col]
			out.consumed[col] = cons + bal
			total, _ := ret[col]
			ret[col] = total + bal
		}
		out.remain = make(map[ledgerstate.Color]uint64) // clear remaining
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

func MakeUTXOInputs(consumables ...*ConsumableOutput) ledgerstate.Inputs {
	ret := make(ledgerstate.Inputs, len(consumables))
	for i, out := range consumables {
		ret[i] = ledgerstate.NewUTXOInput(out.output.ID())
	}
	return ret
}
