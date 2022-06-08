package dashboard

import (
	"github.com/iotaledger/hive.go/generics/lo"

	chat2 "github.com/iotaledger/goshimmer/packages/chat"
	"github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/chat"
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

// Conflict is a JSON serializable conflict.
type Conflict struct {
	ID      string `json:"tx_id"`
	Opinion `json:"opinion"`
}

// Timestamp is a JSON serializable Timestamp.
type Timestamp struct {
	ID      string `json:"msg_id"`
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
		return BasicStringPayload{
			ContentTitle: "address",
			Content:      p.(*faucet.Payload).Address().Base58(),
		}
	case chat2.Type:
		chatPayload := p.(*chat2.Payload)
		return chat.Request{
			From:    chatPayload.From(),
			To:      chatPayload.To(),
			Message: chatPayload.Message(),
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
		deps.Tangle.Ledger.Storage.CachedOutput(refOutputID).Consume(func(output utxo.Output) {
			if typedOutput, ok := output.(devnetvm.Output); ok {
				tp.Transaction.Inputs[i].Output = jsonmodels.NewOutput(typedOutput)
			}
		})
	}

	return
}
