package dashboard

import (
	drngpayload "github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	drngheader "github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	cb "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/marshalutil"
)

type BasicPayload struct {
	ContentTitle string `json:"content_title"`
	Bytes        []byte `json:"bytes"`
}

type DrngPayload struct {
	SubPayloadType byte        `json:"subpayload_type"`
	InstanceId     uint32      `json:"instance_id"`
	SubPayload     interface{} `json:"drngpayload"`
}

type DrngCollectiveBeaconPayload struct {
	Round   uint64 `json:"round"`
	PrevSig []byte `json:"prev_sig"`
	Sig     []byte `json:"sig"`
	Dpk     []byte `json:"dpk"`
}

func ProcessPayload(p payload.Payload) interface{} {
	switch p.Type() {
	case payload.DataType:
		// data payload
		return BasicPayload{
			ContentTitle: "Data",
			Bytes:        p.(*payload.Data).Data(),
		}
	case drngpayload.Type:
		// drng payload
		return processDrngPayload(p)
	default:
		// unknown payload
		return BasicPayload{
			ContentTitle: "Bytes",
			Bytes:        p.Bytes(),
		}
	}
}

func processDrngPayload(p payload.Payload) (dp DrngPayload) {
	var subpayload interface{}
	marshalUtil := marshalutil.New(p.Bytes())
	drngPayload, _ := drngpayload.Parse(marshalUtil)

	switch drngPayload.Header.PayloadType {
	case drngheader.TypeCollectiveBeacon:
		marshalUtil := marshalutil.New(drngPayload.Bytes())
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
			Bytes:        drngPayload.Bytes(),
		}
	}
	return DrngPayload{
		SubPayloadType: drngPayload.Header.PayloadType,
		InstanceId:     drngPayload.Header.InstanceID,
		SubPayload:     subpayload,
	}
}
