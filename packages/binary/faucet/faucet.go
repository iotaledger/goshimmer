package faucet

import (
	"fmt"

	faucetpayload "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	/*
		"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
		"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance"
		"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance/color"
		valuepayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
		payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
		"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer"
		transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
		"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/inputs"
		"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/outputs"
		"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/signatures"
		transferoutputid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transferoutput/id"
	*/)

func IsFaucetReq(txn *message.Transaction) bool {
	return txn.GetPayload().GetType() == faucetpayload.Type
}

func SendFunds(txn *message.Transaction) error {
	addr := txn.GetPayload().(*faucetpayload.Payload).GetAddress()
	// Check address length
	if len(addr) != address.Length {
		return ErrInvalidAddr
	}

	fmt.Println(addr)
	return nil

	// TODO: Send value transfer
}
