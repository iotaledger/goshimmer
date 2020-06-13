package autopeering

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsensusNoConflicts issues valid non-conflicting value objects and then checks
// whether the ledger of every peer reflects the same correct state.
func TestConsensusNoConflicts(t *testing.T) {
	n, err := f.CreateNetwork("consensus_TestConsensusNoConflicts", 4, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	time.Sleep(5 * time.Second)

	// genesis wallet
	genesisSeedBytes, err := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
	require.NoError(t, err, "couldn't decode genesis seed from base58 seed")

	const genesisBalance = 1000000000
	genesisWallet := wallet.New(genesisSeedBytes)
	genesisAddr := genesisWallet.Seed().Address(0)
	genesisOutputID := transaction.NewOutputID(genesisAddr, transaction.GenesisID)

	w := wallet.New()
	addr1 := w.Seed().Address(0)
	addr2 := w.Seed().Address(1)
	const deposit = genesisBalance / 2

	// issue transaction spending from the genesis output
	tx := transaction.New(
		transaction.NewInputs(genesisOutputID),
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			addr1: {{Value: deposit, Color: balance.ColorIOTA}},
			addr2: {{Value: deposit, Color: balance.ColorIOTA}},
		}),
	)
	tx = tx.Sign(signaturescheme.ED25519(*genesisWallet.Seed().KeyPair(0)))
	utilsTx := utils.ParseTransaction(tx)

	txID, err := n.Peers()[0].SendTransaction(tx.Bytes())
	require.NoError(t, err)

	// wait for the transaction to be propagated through the network
	time.Sleep(20 * time.Second)

	// check that each node has the same perception
	for _, p := range n.Peers() {
		// check existence of the transaction we just created
		res, err := p.GetTransactionByID(txID)
		assert.NoError(t, err)
		assert.Len(t, res.Error, 0, "there shouldn't be any error from submitting the valid transaction")
		assert.EqualValues(t, utilsTx.Inputs, res.Transaction.Inputs)
		assert.EqualValues(t, utilsTx.Outputs, res.Transaction.Outputs)
		assert.EqualValues(t, utilsTx.Signature, res.Transaction.Signature)

		// check that genesis UTXO is spent
		utxos, err := p.GetUnspentOutputs([]string{genesisAddr.String()})
		assert.NoError(t, err)
		assert.Len(t, utxos.Error, 0, "there shouldn't be any error from querying UTXOs")
		assert.Len(t, utxos.UnspentOutputs, 0, "genesis address should not have any UTXOs")

		// check UTXOs
		utxos, err = p.GetUnspentOutputs([]string{addr1.String(), addr2.String()})
		assert.Len(t, utxos.UnspentOutputs, 2, "addresses should have UTXOs")
		assert.Equal(t, addr1.String(), utxos.UnspentOutputs[0].Address)
		assert.EqualValues(t, deposit, utxos.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)
		assert.Equal(t, addr2.String(), utxos.UnspentOutputs[1].Address)
		assert.EqualValues(t, deposit, utxos.UnspentOutputs[1].OutputIDs[0].Balances[0].Value)
	}
}
