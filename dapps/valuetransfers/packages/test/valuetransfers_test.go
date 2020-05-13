package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

func TestTangle_AttachPayload(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)

	config.Node.Set(database.CFG_DIRECTORY, dir)

	valueTangle := tangle.New(database.GetBadgerInstance())
	if err := valueTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	ledgerState := ledgerstate.New(valueTangle)

	addressKeyPair1 := ed25519.GenerateKeyPair()
	addressKeyPair2 := ed25519.GenerateKeyPair()
	address1 := address.FromED25519PubKey(addressKeyPair1.PublicKey)
	address2 := address.FromED25519PubKey(addressKeyPair2.PublicKey)

	valueTangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			address1: []*balance.Balance{
				balance.New(balance.ColorIOTA, 337),
			},

			address2: []*balance.Balance{
				balance.New(balance.ColorIOTA, 1000),
			},
		},
	})

	fmt.Printf("Balances on address '%s': %v\n", address1, ledgerState.Balances(address1))

	outputAddress1 := address.Random()
	outputAddress2 := address.Random()

	// attach first spend
	valueTangle.AttachPayload(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(address1, transaction.GenesisID),
			transaction.NewOutputID(address2, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress1: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))

	// attach double spend
	valueTangle.AttachPayload(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(address1, transaction.GenesisID),
			transaction.NewOutputID(address2, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress2: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))

	valueTangle.Shutdown()
	valueTangle.Shutdown()
}
