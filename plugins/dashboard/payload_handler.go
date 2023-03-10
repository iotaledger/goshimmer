package dashboard

import (
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/lo"
)

// BasicPayload contains content title and bytes
// It can be reused with different payload that only contains one field.
type BasicPayload struct {
	ContentTitle string `json:"content_title"`
	Content      []byte `json:"content"`
}

// BasicStringPayload contains content title and string content
type BasicStringPayload struct {
	ContentTitle string `json:"content_title"`
	Content      string `json:"content"`
}

// TransactionPayload contains the transaction information.
type TransactionPayload struct {
	TxID        string                  `json:"txID"`
	Transaction *jsonmodels.Transaction `json:"transaction"`
}

// Essence contains the transaction essence information.
type Essence struct {
	Version           uint8                `json:"version"`
	Timestamp         int                  `json:"timestamp"`
	AccessPledgeID    string               `json:"access_pledge_id"`
	ConsensusPledgeID string               `json:"cons_pledge_id"`
	Inputs            []*jsonmodels.Output `json:"inputs"`
	Outputs           []*jsonmodels.Output `json:"outputs"`
	Data              string               `json:"data"`
}

// Conflict is a JSON serializable conflictSet.
type Conflict struct {
	ID      string `json:"tx_id"`
	Opinion `json:"opinion"`
}

// Timestamp is a JSON serializable Timestamp.
type Timestamp struct {
	ID      string `json:"blk_id"`
	Opinion `json:"opinion"`
}

// Opinion is a JSON serializable opinion.
type Opinion struct {
	Value string `json:"value"`
	Round uint8  `json:"round"`
}

// InputContent contains the inputs of a transaction
type InputContent struct {
	OutputID string    `json:"output_id"`
	Address  string    `json:"address"`
	Balances []Balance `json:"balance"`
}

// OutputContent contains the outputs of a transaction
type OutputContent struct {
	OutputID string    `json:"output_id"`
	Address  string    `json:"address"`
	Balances []Balance `json:"balance"`
}

// Balance contains the amount of specific color token
type Balance struct {
	Value uint64 `json:"value"`
	Color string `json:"color"`
}

// ProcessPayload returns different structs regarding to the
// payload type.
func ProcessPayload(p payload.Payload) interface{} {
	switch p.Type() {
	case payload.GenericDataPayloadType:
		// data payload
		return BasicPayload{
			ContentTitle: "GenericDataPayload",
			Content:      p.(*payload.GenericDataPayload).Blob(),
		}
	case devnetvm.TransactionType:
		return processTransactionPayload(p)
	case faucet.RequestType:
		// faucet payload
		faucetPayload := p.(*faucet.Payload)
		return jsonmodels.FaucetRequest{
			Address:               faucetPayload.Address().Base58(),
			ConsensusManaPledgeID: faucetPayload.ConsensusManaPledgeID().EncodeBase58(),
			AccessManaPledgeID:    faucetPayload.AccessManaPledgeID().EncodeBase58(),
			Nonce:                 faucetPayload.M.Nonce,
		}
	default:
		// unknown payload
		return BasicPayload{
			ContentTitle: "Bytes",
			Content:      lo.PanicOnErr(p.Bytes()),
		}
	}
}

// processTransactionPayload handles Value payload
func processTransactionPayload(p payload.Payload) (tp TransactionPayload) {
	tx := p.(*devnetvm.Transaction)
	tp.TxID = tx.ID().Base58()
	tp.Transaction = jsonmodels.NewTransaction(tx)
	// add consumed inputs
	for i, input := range tx.Essence().Inputs() {
		refOutputID := input.(*devnetvm.UTXOInput).ReferencedOutputID()
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutput(refOutputID).Consume(func(output utxo.Output) {
			if typedOutput, ok := output.(devnetvm.Output); ok {
				tp.Transaction.Inputs[i].Output = jsonmodels.NewOutput(typedOutput)
			}
		})
	}

	return
}
