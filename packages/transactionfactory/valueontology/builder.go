package valueontology

//import (
//	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
//	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/address"
//	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/coloredbalance"
//	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/coloredbalance/color"
//	valuepayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload"
//	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/id"
//	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer"
//	transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/id"
//	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/inputs"
//	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/outputs"
//	transferoutputid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/transferoutput/id"
//	"github.com/iotaledger/goshimmer/packages/transactionfactory"
//)
//
//type Builder struct {
//}
//
//func (b *Builder) BuildPayload(valueTransfer *transfer.Transfer) *payload.Payload {
//	// TODO: do tip selection
//	var valuePayload payload.Payload = valuepayload.New(
//		payloadid.Empty,
//		payloadid.Empty,
//		valueTransfer,
//	)
//
//	transactionfactory.Events.PayloadConstructed.Trigger(&valuePayload)
//
//	return &valuePayload
//}
//
//func (b *Builder) BuildValueTransfer() *transfer.Transfer {
//	// TODO: currently only a dummy transfer
//	valueTransfer := transfer.New(
//		// inputs
//		inputs.New(
//			transferoutputid.New(address.New([]byte("input_address1")), transferid.New([]byte("transfer1"))),
//			transferoutputid.New(address.New([]byte("input_address2")), transferid.New([]byte("transfer2"))),
//		),
//
//		// outputs
//		outputs.New(map[address.Address][]*coloredbalance.ColoredBalance{
//			address.New([]byte("output_address")): {
//				coloredbalance.New(color.IOTA, 1337),
//			},
//		}),
//	)
//
//	return valueTransfer
//}
