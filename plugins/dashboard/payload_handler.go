package dashboard

import (
	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	drngpayload "github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	drngheader "github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	cb "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
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

// DrngPayload contains the subtype of drng payload, instance Id
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
	ParentID0 string          `json:"parent_id_0"`
	ParentID1 string          `json:"parent_id_1"`
	TxID      string          `json:"tx_id"`
	Input     []InputContent  `json:"inputs"`
	Output    []OutputContent `json:"outputs"`
	Data      []byte          `json:"data"`
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
	case payload.DataType:
		// data payload
		return BasicPayload{
			ContentTitle: "Data",
			Content:      p.(*payload.Data).Data(),
		}
	case faucetpayload.Type:
		// faucet payload
		return BasicStringPayload{
			ContentTitle: "address",
			Content:      p.(*faucetpayload.Payload).Address().String(),
		}
	case drngpayload.Type:
		// drng payload
		return processDrngPayload(p)
	case valuepayload.Type:
		return processValuePayload(p)
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
	drngPayload, _ := drngpayload.Parse(marshalUtil)

	switch drngPayload.Header.PayloadType {
	case drngheader.TypeCollectiveBeacon:
		// collective beacon
		marshalUtil := marshalutil.New(p.Bytes())
		cbp, _ := cb.Parse(marshalUtil)
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
		for _, balance := range balances {
			b = append(b, Balance{
				Value: balance.Value,
				Color: balance.Color.String(),
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
		ParentID0: v.TrunkID().String(),
		ParentID1: v.BranchID().String(),
		TxID:      v.Transaction().ID().String(),
		Input:     inputs,
		Output:    outputs,
		Data:      v.Transaction().GetDataPayload(),
	}
}
