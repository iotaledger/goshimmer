package value

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

var maxBookedAwaitTime = 5 * time.Second

// ParseTransaction handle transaction json object.
func ParseTransaction(t *ledgerstate.Transaction) (txn Transaction) {
	var inputs []string
	var outputs []Output
	// process inputs
	for _, input := range t.Essence().Inputs() {
		inputs = append(inputs, input.Base58())
	}

	// process outputs
	for _, output := range t.Essence().Outputs() {
		var balances []Balance
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			balances = append(balances, Balance{
				Value: int64(balance),
				Color: color.String(),
			})
			return true
		})
		outputs = append(outputs, Output{
			Address:  output.Address().Base58(),
			Balances: balances,
		})
	}
	return Transaction{
		Inputs:      inputs,
		Outputs:     outputs,
		Signature:   t.UnlockBlocks().Bytes(),
		DataPayload: t.Essence().Bytes(),
	}
}

// Transaction holds the information of a transaction.
type Transaction struct {
	Inputs      []string `json:"inputs"`
	Outputs     []Output `json:"outputs"`
	Signature   []byte   `json:"signature"`
	DataPayload []byte   `json:"data_payload"`
}

// OutputID holds the output id and its inclusion state
type OutputID struct {
	ID             string         `json:"id"`
	Balances       []Balance      `json:"balances"`
	InclusionState InclusionState `json:"inclusion_state"`
}

// UnspentOutput holds the address and the corresponding unspent output ids
type UnspentOutput struct {
	Address   string     `json:"address"`
	OutputIDs []OutputID `json:"output_ids"`
}

// Output consists an address and balances
type Output struct {
	Type     int8      `json:"type"`
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

// Signature defines the struct of a signature.
type Signature struct {
	Version   byte   `json:"version"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
}
