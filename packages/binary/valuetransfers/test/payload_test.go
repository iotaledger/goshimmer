package test

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/coloredbalance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/coloredbalance/color"
	transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/inputs"
	transferoutputid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/output/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/outputs"
)

func TestPayload(t *testing.T) {
	createdPayload := payload.NewPayload(
		payloadid.Empty,
		payloadid.Empty,
		transfer.New(
			inputs.New(
				transferoutputid.New(address.New([]byte("test")), transferid.New([]byte("test"))),
				transferoutputid.New(address.New([]byte("test")), transferid.New([]byte("test1"))),
			),

			outputs.New(map[address.Address][]*coloredbalance.ColoredBalance{
				address.New([]byte("output_address")): {
					coloredbalance.New(color.COLOR_IOTA, 1337),
				},
			}),
		),
	)

	fmt.Println(createdPayload.Bytes())

	restoredTransfer, err, _ := valuetransfers.TransferFromBytes(sourceTransfer.Bytes())
	if err != nil {
		t.Error(err)

		return
	}

	fmt.Println(sourceTransfer)
	fmt.Println(restoredTransfer)

	fmt.Println(restoredTransfer.GetId())
	fmt.Println(sourceTransfer.GetId())
}
