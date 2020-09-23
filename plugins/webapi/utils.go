package webapi

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// Message contains information about a given message.
type Message struct {
	Metadata        `json:"metadata,omitempty"`
	ID              string `json:"ID,omitempty"`
	Parent1ID       string `json:"parent1Id,omitempty"`
	Parent2ID       string `json:"parent2Id,omitempty"`
	IssuerPublicKey string `json:"issuerPublicKey,omitempty"`
	IssuingTime     int64  `json:"issuingTime,omitempty"`
	SequenceNumber  uint64 `json:"sequenceNumber,omitempty"`
	Payload         []byte `json:"payload,omitempty"`
	Signature       string `json:"signature,omitempty"`
}

// Metadata contains metadata information of a message.
type Metadata struct {
	Solid              bool  `json:"solid,omitempty"`
	SolidificationTime int64 `json:"solidificationTime,omitempty"`
}

// ValueObject holds the info of a ValueObject
type ValueObject struct {
	Parent1       string      `json:"parent_1,omitempty"`
	Parent2       string      `json:"parent_2,omitempty"`
	ID            string      `json:"id"`
	Tip           bool        `json:"tip,omitempty"`
	Solid         bool        `json:"solid"`
	Liked         bool        `json:"liked"`
	Confirmed     bool        `json:"confirmed"`
	Rejected      bool        `json:"rejected"`
	BranchID      string      `json:"branch_id"`
	TransactionID string      `json:"transaction_id"`
	Transaction   Transaction `json:"transaction,omitempty"`
}

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

// UnspentOutput holds the address and the corresponding unspent output ids
type UnspentOutput struct {
	Address   string     `json:"address"`
	OutputIDs []OutputID `json:"output_ids"`
}

// OutputID holds the output id and its inclusion state
type OutputID struct {
	ID             string         `json:"id"`
	Balances       []Balance      `json:"balances"`
	InclusionState InclusionState `json:"inclusion_state"`
}

// Signature defines the struct of a signature.
type Signature struct {
	Version   byte   `json:"version"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
}
