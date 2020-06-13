package tangle

import (
	"bytes"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

func TestLoadSnapshot(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			address.Random(): []*balance.Balance{
				balance.New(balance.ColorIOTA, 337),
			},

			address.Random(): []*balance.Balance{
				balance.New(balance.ColorIOTA, 1000),
				balance.New(balance.ColorIOTA, 1000),
			},
		},
	}
	tangle.LoadSnapshot(snapshot)

	// check whether outputs can be retrieved from tangle
	for addr, balances := range snapshot[transaction.GenesisID] {
		cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(addr, transaction.GenesisID))
		cachedOutput.Consume(func(output *Output) {
			assert.Equal(t, addr, output.Address())
			assert.ElementsMatch(t, balances, output.Balances())
			assert.True(t, output.Solid())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID())
		})
	}
}

func TestSnapshotMarshalUnmarshal(t *testing.T) {
	const genesisBalance = 1000000000
	seed := wallet.NewSeed()
	genesisAddr := seed.Address(GENESIS)

	snapshot := Snapshot{
		transaction.GenesisID: {
			genesisAddr: {
				balance.New(balance.ColorIOTA, genesisBalance),
			},
		},
	}

	// includes txs count
	const int64ByteSize = 8
	expectedLength := int64ByteSize
	for _, addresses := range snapshot {
		// tx id
		expectedLength += transaction.IDLength
		// addr count
		expectedLength += int64ByteSize
		for _, balances := range addresses {
			// addr
			expectedLength += address.Length
			// balance count
			expectedLength += int64ByteSize
			// balances
			expectedLength += len(balances) * (int64ByteSize + balance.ColorLength)
		}
	}

	var buf bytes.Buffer
	written, err := snapshot.WriteTo(&buf)
	assert.NoError(t, err, "writing the snapshot to the buffer should succeed")
	assert.EqualValues(t, expectedLength, written, "written byte count should match the expected count")

	snapshotFromBytes := Snapshot{}
	read, err := snapshotFromBytes.ReadFrom(&buf)
	assert.NoError(t, err, "expected no error from reading valid snapshot bytes")
	assert.EqualValues(t, expectedLength, read, "read byte count should match the expected count")

	// check that the source and unmarshaled snapshot are equivalent
	assert.Equal(t, snapshot, snapshotFromBytes)
}
