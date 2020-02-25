package test

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/valuetangle"
)

func TestPayload(t *testing.T) {
	transfer := valuetangle.NewTransfer()
	fmt.Println(transfer.GetId())

	payload := valuetangle.NewPayload(valuetangle.EmptyPayloadId, valuetangle.EmptyPayloadId, transfer)
	fmt.Println(payload.GetId())
}
