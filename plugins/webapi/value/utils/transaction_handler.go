package utils

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// ParseTransaction handle transaction json object.
func ParseTransaction(t *transaction.Transaction) (txn Transaction) {
	var inputs []string
	var outputs []Output
	// process inputs
	t.Inputs().ForEachAddress(func(currentAddress address.Address) bool {
		inputs = append(inputs, currentAddress.String())
		return true
	})

	// process outputs: address + balance
	t.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		var b []Balance
		for _, balance := range balances {
			b = append(b, Balance{
				Value: balance.Value,
				Color: balance.Color.String(),
			})
		}
		t := Output{
			Address:  address.String(),
			Balances: b,
		}
		outputs = append(outputs, t)

		return true
	})

	return Transaction{
		Inputs:      inputs,
		Outputs:     outputs,
		Signature:   t.SignatureBytes(),
		DataPayload: t.GetDataPayload(),
	}
}

// Transaction holds the information of a transaction.
type Transaction struct {
	Inputs      []string `json:"inputs"`
	Outputs     []Output `json:"outputs"`
	Signature   []byte   `json:"signature"`
	DataPayload []byte   `json:"data_payload"`
}

// Output consists an address and balances
type Output struct {
	Address  string    `json:"address"`
	Balances []Balance `json:"balances"`
}

// Balance holds the value and the color of token
type Balance struct {
	Value int64  `json:"value"`
	Color string `json:"color"`
}

// InclusionState represents the different states of an OutputID
type InclusionState struct {
	Solid       bool `json:"solid,omitempty"`
	Confirmed   bool `json:"confirmed,omitempty"`
	Rejected    bool `json:"rejected,omitempty"`
	Liked       bool `json:"liked,omitempty"`
	Conflicting bool `json:"conflicting,omitempty"`
	Finalized   bool `json:"finalized,omitempty"`
	Preferred   bool `json:"preferred,omitempty"`
}
