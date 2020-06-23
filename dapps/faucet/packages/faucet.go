package faucet

import (
	"fmt"

	faucetpayload "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/iotaledger/goshimmer/plugins/issuer"
)

const (
	// TODO: Modify here before release
	TokenPerRequest = 1000
)

var (
	// the faucet wallet
	faucetWallet *wallet.Wallet
	// the faucet seed, just an example for testing lol
	faucetSeed = []byte{95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113,
		104, 71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230}
)

// ConfigureFaucet restore wallet with faucet seed
func ConfigureFaucet() {
	faucetWallet = wallet.New(faucetSeed)
}

// IsFaucetReq checks if the message is faucet payload
func IsFaucetReq(msg *message.Message) bool {
	return msg.Payload().Type() == faucetpayload.Type
}

// SendFunds sends IOTA token to the address in faucet payload
func SendFunds(msg *message.Message) (txID string, err error) {
	addr := msg.Payload().(*faucetpayload.Payload).Address()
	// Check address length
	if len(addr) != address.Length {
		return "", ErrInvalidAddr
	}

	// TODO: remove me
	fmt.Println("process faucet request: ", addr)

	// get the output ids for inputs and remain balance for outputs
	outputIds, remain := getUnspentOutputID()

	tx := transaction.New(
		// inputs
		transaction.NewInputs(outputIds...),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			addr: {
				balance.New(balance.ColorIOTA, TokenPerRequest),
			},
		}),
	)

	// add remain address if needed
	if remain > 0 {
		remainAddr := getUnusedAddress()
		tx.Outputs().Add(remainAddr, []*balance.Balance{balance.New(balance.ColorIOTA, remain)})
	}

	// prepare value payload with value factory
	payload := valuetransfers.ValueObjectFactory().IssueTransaction(tx)

	// attach to message layer
	_, err = issuer.IssuePayload(payload)
	if err != nil {
		return "", err
	}

	return tx.ID().String(), nil
}

// getUnspentOutputID iterate unspent outputs until the value of balance reaches TokenPerRequest
// the remaining value is also calculated for outputs
func getUnspentOutputID() (outputIds []transaction.OutputID, remain int64) {
	var total int64 = TokenPerRequest
	var i uint64

	// get a list of address for inputs
	for i = 0; total > 0; i++ {
		addr := faucetWallet.Seed().Address(i)

		valuetransfers.Tangle.OutputsOnAddress(addr).Consume(func(output *tangle.Output) {
			if output.ConsumerCount() == 0 && total > 0 {
				var val int64 = 0
				for _, coloredBalance := range output.Balances() {
					val += coloredBalance.Value
				}

				// get unspent output ids and check if it's conflict
				if val <= total {
					total -= val
				} else {
					remain = val - total
					total = 0
				}
				outputIds = append(outputIds, output.ID())
			}
		})
	}

	return
}

// getUnusedAddress generates a unused address of the faucet seed
func getUnusedAddress() address.Address {
	var index uint64
	for index = 0; ; index++ {
		addr := faucetWallet.Seed().Address(index)
		cachedOutputs := valuetransfers.Tangle.OutputsOnAddress(addr)
		if len(cachedOutputs) > 0 {
			cachedOutputs.Release()
			continue
		} else {
			return addr
		}
	}
}
