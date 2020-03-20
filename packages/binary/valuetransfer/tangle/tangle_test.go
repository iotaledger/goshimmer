package tangle

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance/color"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/payloadmetadata"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transfer"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

func TestTangle_AttachPayload(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)

	config.Node.Set(database.CFG_DIRECTORY, dir)

	tangle := New(database.GetBadgerInstance(), []byte("TEST_BINARY_TANGLE"))
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	tangle.Events.PayloadSolid.Attach(events.NewClosure(func(payload *payload.CachedObject, metadata *payloadmetadata.CachedObject) {
		fmt.Println(payload.Unwrap())

		payload.Release()
		metadata.Release()
	}))

	addressKeyPair1 := ed25119.GenerateKeyPair()
	addressKeyPair2 := ed25119.GenerateKeyPair()

	transferId1, _ := transfer.FromBase58("8opHzTAnfzRpPEx21XtnrVTX28YQuCpAjcn1PczScKh")
	transferId2, _ := transfer.FromBase58("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM")

	tangle.AttachPayload(payload.New(id.Genesis, id.Genesis, transfer.New(
		transfer.NewInputs(
			transfer.NewOutputId(address.FromED25519PubKey(addressKeyPair1.PublicKey), transferId1),
			transfer.NewOutputId(address.FromED25519PubKey(addressKeyPair2.PublicKey), transferId2),
		),

		transfer.NewOutputs(map[address.Address][]*coloredbalance.ColoredBalance{
			address.Random(): {
				coloredbalance.New(color.IOTA, 1337),
			},
		}),
	)))

	tangle.Shutdown()
}
