package value

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/mr-tron/base58/base58"
)

var maxBookedAwaitTime = 5 * time.Second

// ParseTransaction handle transaction json object.
func ParseTransaction(tx *ledgerstate.Transaction) (txn Transaction) {
	// process inputs
	inputs := make([]Input, len(tx.Essence().Inputs()))
	for i, input := range tx.Essence().Inputs() {
		inputs[i] = Input{
			ConsumedOutputID: input.Base58(),
		}
	}

	// process outputs
	outputs := make([]Output, len(tx.Essence().Outputs()))
	for i, output := range tx.Essence().Outputs() {
		var balances []Balance
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			balances = append(balances, Balance{
				Color: color.String(),
				Value: int64(balance),
			})
			return true
		})
		outputs[i] = Output{
			Type:     output.Type(),
			Address:  output.Address().Base58(),
			Balances: balances,
		}
	}

	// process unlock blocks
	unlockBlocks := make([]UnlockBlock, len(tx.UnlockBlocks()))
	for i, unlockBlock := range tx.UnlockBlocks() {
		ub := UnlockBlock{
			Type: unlockBlock.Type(),
		}
		switch unlockBlock.Type() {
		case ledgerstate.SignatureUnlockBlockType:
			signature, _, _ := ledgerstate.SignatureFromBytes(unlockBlock.Bytes())
			ub.SignatureType = signature.Type()
			switch signature.Type() {
			case ledgerstate.ED25519SignatureType:
				signature, _, _ := ledgerstate.ED25519SignatureFromBytes(signature.Bytes())
				ub.PublicKey = signature.PublicKey.String()
				ub.Signature = signature.Signature.String()

			case ledgerstate.BLSSignatureType:
				signature, _, _ := ledgerstate.BLSSignatureFromBytes(signature.Bytes())
				ub.Signature = signature.Signature.String()
			}
		case ledgerstate.ReferenceUnlockBlockType:
			referenceUnlockBlock, _, _ := ledgerstate.ReferenceUnlockBlockFromBytes(unlockBlock.Bytes())
			ub.ReferencedIndex = referenceUnlockBlock.ReferencedIndex()
		}

		unlockBlocks[i] = ub
	}

	dataPayload := []byte{}
	if tx.Essence().Payload() != nil {
		dataPayload = tx.Essence().Payload().Bytes()
	}

	return Transaction{
		Version:           tx.Essence().Version(),
		Timestamp:         tx.Essence().Timestamp().Unix(),
		AccessPledgeID:    base58.Encode(tx.Essence().AccessPledgeID().Bytes()),
		ConsensusPledgeID: base58.Encode(tx.Essence().ConsensusPledgeID().Bytes()),
		Inputs:            inputs,
		Outputs:           outputs,
		UnlockBlocks:      unlockBlocks,
		DataPayload:       dataPayload,
	}
}

// TransactionMetadata holds the information of a transaction metadata.
type TransactionMetadata struct {
	BranchID           string `json:"branchID"`
	Solid              bool   `json:"solid"`
	SolidificationTime int64  `json:"solidificationTime"`
	Finalized          bool   `json:"finalized"`
	LazyBooked         bool   `json:"lazyBooked"`
}

// Transaction holds the information of a transaction.
type Transaction struct {
	Version           ledgerstate.TransactionEssenceVersion `json:"version"`
	Timestamp         int64                                 `json:"timestamp"`
	AccessPledgeID    string                                `json:"accessPledgeID"`
	ConsensusPledgeID string                                `json:"consensusPledgeID"`
	Inputs            []Input                               `json:"inputs"`
	Outputs           []Output                              `json:"outputs"`
	UnlockBlocks      []UnlockBlock                         `json:"unlockBlocks"`
	DataPayload       []byte                                `json:"dataPayload"`
}

// Metadata holds metadata about the output.
type Metadata struct {
	Timestamp time.Time `json:"timestamp"`
}

// OutputID holds the output id and its inclusion state
type OutputID struct {
	ID             string         `json:"id"`
	Balances       []Balance      `json:"balances"`
	InclusionState InclusionState `json:"inclusion_state"`
	Metadata       Metadata       `json:"output_metadata"`
}

// UnspentOutput holds the address and the corresponding unspent output ids
type UnspentOutput struct {
	Address   string     `json:"address"`
	OutputIDs []OutputID `json:"output_ids"`
}

// Input holds the consumedOutputID
type Input struct {
	ConsumedOutputID string `json:"consumedOutputID"`
}

// Output consists an address and balances
type Output struct {
	Type     ledgerstate.OutputType `json:"type"`
	Address  string                 `json:"address"`
	Balances []Balance              `json:"balances"`
}

// Balance holds the value and the color of token
type Balance struct {
	Color string `json:"color"`
	Value int64  `json:"value"`
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

// UnlockBlock defines the struct of a signature.
type UnlockBlock struct {
	Type            ledgerstate.UnlockBlockType `json:"type"`
	ReferencedIndex uint16                      `json:"referencedIndex,omitempty"`
	SignatureType   ledgerstate.SignatureType   `json:"signatureType,omitempty"`
	PublicKey       string                      `json:"publicKey,omitempty"`
	Signature       string                      `json:"signature,omitempty"`
}
