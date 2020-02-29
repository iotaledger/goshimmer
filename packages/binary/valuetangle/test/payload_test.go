package test

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/valuetangle"
)

func TestPayload(t *testing.T) {
	inputs := valuetangle.NewTransferInputs().Add(
		valuetangle.NewTransferOutputId(valuetangle.NewAddress([]byte("test")), valuetangle.NewTransferId([]byte("test"))),
	).Add(
		valuetangle.NewTransferOutputId(valuetangle.NewAddress([]byte("test")), valuetangle.NewTransferId([]byte("test1"))),
	)

	transfer := valuetangle.NewTransfer(inputs)
	bytes, err := transfer.MarshalBinary()
	if err != nil {
		t.Error(err)

		return
	}

	var restoredTransfer valuetangle.Transfer
	fmt.Println(restoredTransfer.UnmarshalBinary(bytes))
	fmt.Println(bytes, nil)
	fmt.Println(restoredTransfer.MarshalBinary())
}
