package faucet

import (
	"fmt"

	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
    /*
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
    */
)

func IsFaucetReq(msg *message.Message) bool {
	return msg.Payload().Type() == faucetpayload.Type
}

func SendFunds(msg *message.Message) error {
	addr := msg.Payload().(*faucetpayload.Payload).Address()
	// Check address length
	if len(addr) != address.Length {
		return ErrInvalidAddr
	}

	fmt.Println(addr)
	return nil

	// TODO: Send value transfer
    // 1. prepare inputs: get unspent output ID
    // 2. prepare outputs: request address + remain address (how to get node's public key?)
    // 3. prepare valueObject with value factory
    /*
	valueObject := transaction.New(
		// inputs
		transaction.NewInputs(
			transaction.NewOutputID(address.Random(), transaction.RandomID()),
			transaction.NewOutputID(address.Random(), transaction.RandomID()),
		),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			addr: {
				balance.New(balance.ColorIOTA, 1000000),
			},
		}),
	)
    */

	// 4. message factory 
}
