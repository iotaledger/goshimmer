package dashboard

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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

// DrngPayload contains the subtype of drng payload, instance ID
// and the subpayload
type DrngPayload struct {
	SubPayloadType byte        `json:"subpayload_type"`
	InstanceID     uint32      `json:"instance_id"`
	SubPayload     interface{} `json:"drngpayload"`
}

// DrngCollectiveBeaconPayload is the subpayload of DrngPayload.
type DrngCollectiveBeaconPayload struct {
	Round   uint64 `json:"round"`
	PrevSig []byte `json:"prev_sig"`
	Sig     []byte `json:"sig"`
	Dpk     []byte `json:"dpk"`
}

// TransactionPayload contains the transaction information.
type TransactionPayload struct {
	TxID        string                   `json:"txID"`
	Transaction *jsonmodels2.Transaction `json:"transaction"`
}

// Essence contains the transaction essence information.
type Essence struct {
	Version           uint8                 `json:"version"`
	Timestamp         int                   `json:"timestamp"`
	AccessPledgeID    string                `json:"access_pledge_id"`
	ConsensusPledgeID string                `json:"cons_pledge_id"`
	Inputs            []*jsonmodels2.Output `json:"inputs"`
	Outputs           []*jsonmodels2.Output `json:"outputs"`
	Data              string                `json:"data"`
}

// StatementPayload is a JSON serializable statement payload.
type StatementPayload struct {
	Conflicts  []Conflict  `json:"conflicts"`
	Timestamps []Timestamp `json:"timestamps"`
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
	case ledgerstate.TransactionType:
		return processTransactionPayload(p)
	case statement.StatementType:
		return processStatementPayload(p)
	case faucet.Type:
		// faucet payload
		return BasicStringPayload{
			ContentTitle: "address",
			Content:      p.(*faucet.Request).Address().Base58(),
		}
	case drng.PayloadType:
		// drng payload
		return processDrngPayload(p)
	default:
		// unknown payload
		return BasicPayload{
			ContentTitle: "Bytes",
			Content:      p.Bytes(),
		}
	}
}

// processDrngPayload handles the subtypes of Drng payload
func processDrngPayload(p payload.Payload) (dp DrngPayload) {
	var subpayload interface{}
	marshalUtil := marshalutil.New(p.Bytes())
	drngPayload, _ := drng.PayloadFromMarshalUtil(marshalUtil)

	switch drngPayload.Header.PayloadType {
	case drng.TypeCollectiveBeacon:
		// collective beacon
		marshalUtil := marshalutil.New(p.Bytes())
		cbp, _ := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
		subpayload = DrngCollectiveBeaconPayload{
			Round:   cbp.Round,
			PrevSig: cbp.PrevSignature,
			Sig:     cbp.Signature,
			Dpk:     cbp.Dpk,
		}
	default:
		subpayload = BasicPayload{
			ContentTitle: "bytes",
			Content:      drngPayload.Bytes(),
		}
	}
	return DrngPayload{
		SubPayloadType: drngPayload.Header.PayloadType,
		InstanceID:     drngPayload.Header.InstanceID,
		SubPayload:     subpayload,
	}
}

// processTransactionPayload handles Value payload
func processTransactionPayload(p payload.Payload) (tp TransactionPayload) {
	tx, _, err := ledgerstate.TransactionFromBytes(p.Bytes())
	if err != nil {
		return
	}

	tp.TxID = tx.ID().Base58()
	tp.Transaction = jsonmodels2.NewTransaction(tx)
	// add consumed inputs
	for i, input := range tx.Essence().Inputs() {
		refOutputID := input.(*ledgerstate.UTXOInput).ReferencedOutputID()
		messagelayer.Tangle().LedgerState.CachedOutput(refOutputID).Consume(func(output ledgerstate.Output) {
			tp.Transaction.Inputs[i].Output = jsonmodels2.NewOutput(output)
		})
	}

	return
}

func processStatementPayload(p payload.Payload) (sp StatementPayload) {
	tmp := p.(*statement.Statement)

	for _, c := range tmp.Conflicts {
		sc := Conflict{
			ID: c.ID.String(),
			Opinion: Opinion{
				Value: c.Value.String(),
				Round: c.Round,
			},
		}
		sp.Conflicts = append(sp.Conflicts, sc)
	}
	for _, t := range tmp.Timestamps {
		st := Timestamp{
			ID: t.ID.Base58(),
			Opinion: Opinion{
				Value: t.Value.String(),
				Round: t.Round,
			},
		}
		sp.Timestamps = append(sp.Timestamps, st)
	}
	return
}
