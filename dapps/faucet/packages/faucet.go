package faucet

import (
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
	// TokenPerRequest represents the amount of token to be sent per faucet request
	TokenPerRequest = 1000
)

var (
	// the faucet wallet
	faucetWallet *wallet.Wallet
	// the faucet seed, just an example for testing lol
	faucetSeed = []byte{95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113,
		104, 71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230}
)

// ConfigureFaucet restores the faucet wallet with faucet seed.
// TODO: should be part of a struct constructor instead of a function.
func ConfigureFaucet() {
	faucetWallet = wallet.New(faucetSeed)
}

// IsFaucetReq checks if the message is faucet payload.
func IsFaucetReq(msg *message.Message) bool {
	return msg.Payload().Type() == faucetpayload.Type
}

// SendFunds sends IOTA tokens to the address from faucet request.
func SendFunds(msg *message.Message) (txID string, err error) {
	addr := msg.Payload().(*faucetpayload.Payload).Address()

	// get the output ids for the inputs and remainder balance
	outputIds, remainder := getUnspentOutputID()

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

	// add remainder address if needed
	if remainder > 0 {
		remainAddr := getUnusedAddress()
		tx.Outputs().Add(remainAddr, []*balance.Balance{balance.New(balance.ColorIOTA, remainder)})
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

// getUnspentOutputID iterates UTXOs until the TokenPerRequest threshold is reached.
// this function also returns the remainder balance for the given outputs.
func getUnspentOutputID() (outputIds []transaction.OutputID, remainder int64) {
	var total int64 = TokenPerRequest
	var i uint64

	// get a list of address for inputs
	for i = 0; total > 0; i++ {
		addr := faucetWallet.Seed().Address(i)
		valuetransfers.Tangle.OutputsOnAddress(addr).Consume(func(output *tangle.Output) {
			if output.ConsumerCount() > 0 || total == 0 {
				return
			}

			var val int64
			for _, coloredBalance := range output.Balances() {
				val += coloredBalance.Value
			}

			// get unspent output ids and check if it's conflict
			if val <= total {
				total -= val
			} else {
				remainder = val - total
				total = 0
			}
			outputIds = append(outputIds, output.ID())
		})
	}

	return
}

// getUnusedAddress generates an unused address from the faucet seed.
func getUnusedAddress() address.Address {
	var index uint64
	for index = 0; ; index++ {
		addr := faucetWallet.Seed().Address(index)
		cachedOutputs := valuetransfers.Tangle.OutputsOnAddress(addr)
		if len(cachedOutputs) == 0 {
			// unused address
			cachedOutputs.Release()
			return addr
		}
		cachedOutputs.Release()
	}
}
