package faucet

import (
	"fmt"

	faucetpayload "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"

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

func IsFaucetReq(msg *message.Message) bool {
	return msg.GetPayload().Type() == faucetpayload.Type
}

func SendFunds(msg *message.Message) error {
	addr := msg.GetPayload().(*faucetpayload.Payload).Address()
	// Check address length
	if len(addr) != address.Length {
		return ErrInvalidAddr
	}

	fmt.Println(addr)
	return nil

	// TODO: Send value transfer
}
