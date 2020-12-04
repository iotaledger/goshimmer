package dashboard

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	syncbeaconpayload "github.com/iotaledger/goshimmer/plugins/syncbeacon/payload"
	"github.com/iotaledger/hive.go/marshalutil"
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

// SyncBeaconPayload contains sent time of a sync beacon.
type SyncBeaconPayload struct {
	SentTime int64 `json:"sent_time"`
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

// ValuePayload contains the transaction information
type ValuePayload struct {
	ID        string          `json:"payload_id"`
	Parent1ID string          `json:"parent1_id"`
	Parent2ID string          `json:"parent2_id"`
	TxID      string          `json:"tx_id"`
	Input     []InputContent  `json:"inputs"`
	Output    []OutputContent `json:"outputs"`
	Data      []byte          `json:"data"`
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
	Address string `json:"address"`
}

// OutputContent contains the outputs of a transaction
type OutputContent struct {
	Address  string    `json:"address"`
	Balances []Balance `json:"balance"`
}

// Balance contains the amount of specific color token
type Balance struct {
	Value int64  `json:"value"`
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
	case valuepayload.Type:
		return processValuePayload(p)
	case statement.StatementType:
		return processStatementPayload(p)
	case faucet.Type:
		// faucet payload
		return BasicStringPayload{
			ContentTitle: "address",
			Content:      p.(*faucet.Request).Address().String(),
		}
	case drng.PayloadType:
		// drng payload
		return processDrngPayload(p)
	case syncbeaconpayload.Type:
		// sync beacon payload
		return processSyncBeaconPayload(p)
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

// processDrngPayload handles the subtypes of Drng payload
func processSyncBeaconPayload(p payload.Payload) (dp SyncBeaconPayload) {
	syncBeaconPayload, ok := p.(*syncbeaconpayload.Payload)
	if !ok {
		log.Info("could not cast payload to sync beacon object")
		return
	}

	return SyncBeaconPayload{
		SentTime: syncBeaconPayload.SentTime(),
	}
}

// processValuePayload handles Value payload
func processValuePayload(p payload.Payload) (vp ValuePayload) {
	marshalUtil := marshalutil.New(p.Bytes())
	v, _ := valuepayload.Parse(marshalUtil)

	var inputs []InputContent
	var outputs []OutputContent

	// TODO: retrieve balance
	v.Transaction().Inputs().ForEachAddress(func(currentAddress address.Address) bool {
		inputs = append(inputs, InputContent{Address: currentAddress.String()})
		return true
	})

	// Get outputs address and balance
	v.Transaction().Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		var b []Balance
		for _, bal := range balances {
			color := bal.Color.String()
			if bal.Color == balance.ColorNew {
				color = v.Transaction().ID().String()
			}

			b = append(b, Balance{
				Value: bal.Value,
				Color: color,
			})
		}
		t := OutputContent{
			Address:  address.String(),
			Balances: b,
		}
		outputs = append(outputs, t)

		return true
	})

	return ValuePayload{
		ID:        v.ID().String(),
		Parent1ID: v.Parent1ID().String(),
		Parent2ID: v.Parent2ID().String(),
		TxID:      v.Transaction().ID().String(),
		Input:     inputs,
		Output:    outputs,
		Data:      v.Transaction().GetDataPayload(),
	}
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
			ID: t.ID.String(),
			Opinion: Opinion{
				Value: t.Value.String(),
				Round: t.Round,
			},
		}
		sp.Timestamps = append(sp.Timestamps, st)
	}
	return
}
