package tests

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DataMessageSent defines a struct to identify from which issuer a data message was sent.
type DataMessageSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

type Shutdowner interface {
	Shutdown() error
}

// SendDataMessagesOnRandomPeer sends data messages on a random peer and saves the sent message to a map.
func SendDataMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, numMessages int, idsMap ...map[string]DataMessageSent) map[string]DataMessageSent {
	var ids map[string]DataMessageSent
	if len(idsMap) > 0 {
		ids = idsMap[0]
	} else {
		ids = make(map[string]DataMessageSent, numMessages)
	}

	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test%d", i))

		peer := peers[rand.Intn(len(peers))]
		id, sent := SendDataMessage(t, peer, data, i)

		ids[id] = sent
	}

	return ids
}

// SendDataMessage sends a data message on a given peer and returns the id and a DataMessageSent struct.
func SendDataMessage(t *testing.T, peer *framework.Peer, data []byte, number int) (string, DataMessageSent) {
	id, err := peer.Data(data)
	require.NoErrorf(t, err, "Could not send message on %s", peer.String())

	sent := DataMessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewData(data).Bytes(),
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return id, sent
}

// CheckForMessageIds performs checks to make sure that all peers received all given messages defined in ids.
func CheckForMessageIds(t *testing.T, peers []*framework.Peer, ids map[string]DataMessageSent, checkSynchronized bool) {
	var idsSlice []string
	for id := range ids {
		idsSlice = append(idsSlice, id)
	}

	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			require.True(t, info.Synced)
		}

		resp, err := peer.FindMessageByID(idsSlice)
		require.NoError(t, err)

		// check that all messages are present in response
		respIDs := make([]string, len(resp.Messages))
		for i, msg := range resp.Messages {
			respIDs[i] = msg.ID
		}
		assert.ElementsMatchf(t, idsSlice, respIDs, "messages do not match sent in %s", peer.String())

		// check for general information
		for _, msg := range resp.Messages {
			msgSent := ids[msg.ID]

			assert.Equalf(t, msgSent.issuerPublicKey, msg.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Equalf(t, msgSent.data, msg.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Truef(t, msg.Metadata.Solid, "messageID=%s, issuer=%s not solid in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
		}
	}
}

// SendValueMessage sends a value message on a given peer and returns the transaction ID.
func SendValueMessagesOnFaucet(t *testing.T, peers []*framework.Peer) (txId string, addrBalance map[string]int64) {
	faucetPeer := peers[0]
	sigScheme := signaturescheme.ED25519(*faucetPeer.Seed().KeyPair(0))
	// send the same funds to the original address
	remainAddr := faucetPeer.Seed().Address(0)
	var sentValue int64 = 0

	// prepare inputs
	unspentOutputs, err := faucetPeer.GetUnspentOutputs([]string{framework.SnapShotFirstAddress})
	require.NoErrorf(t, err, "Could not get unspent outputs on %s", faucetPeer.String())
	value := unspentOutputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	out, err := transaction.OutputIDFromBase58(unspentOutputs.UnspentOutputs[0].OutputIDs[0].ID)
	require.NoErrorf(t, err, "Invalid unspent outputs ID on %s", faucetPeer.String())
	inputs := transaction.NewInputs([]transaction.OutputID{out}...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}
	addrBalance = make(map[string]int64)

	// send funds to other peers, skip faucet node
	for i := 1; i < len(peers); i++ {
		addr := peers[i].Seed().Address(0)

		// set balances
		balances := []*balance.Balance{balance.New(balance.ColorIOTA, 100)}
		outmap[addr] = balances
		sentValue += 100
		// set return map
		addrBalance[addr.String()] = 100
	}
	outputs := transaction.NewOutputs(outmap)

	// handle remain address
	outputs.Add(remainAddr, []*balance.Balance{balance.New(balance.ColorIOTA, value-sentValue)})
	addrBalance[remainAddr.String()] = value - sentValue

	// sign transaction
	txn := transaction.New(inputs, outputs).Sign(sigScheme)

	// send transaction
	txId, err = faucetPeer.SendTransaction(txn.Bytes())
	require.NoErrorf(t, err, "Could not send transaction on %s", faucetPeer.String())

	return
}

// SendDataMessagesOnRandomPeer sends data messages on a random peer and saves the sent message to a map.
func SendValueMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, addrBalance map[string]int64, numMessages int) {
	addrMap := make(map[int]string)
	for i, p := range peers {
		addrMap[i] = p.Seed().Address(0).String()
	}

	for i := 0; i < numMessages; i++ {
		from := rand.Intn(len(peers))
		to := rand.Intn(len(peers))
		sentValue := SendValueMessages(t, peers[from], peers[to], addrBalance)

		// update balance
		addrBalance[addrMap[from]] -= sentValue
		addrBalance[addrMap[to]] += sentValue

		if sentValue == 0 {
			i--
		}
		time.Sleep(5 * time.Second)
	}

	return
}

// SendValueMessage sends a value message from and to a given peer and returns the transaction ID.
// for testing convenience, the same addresses are used in each round
func SendValueMessages(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]int64) (sent int64) {
	sigScheme := signaturescheme.ED25519(*from.Seed().KeyPair(0))
	inputAddr := from.Seed().Address(0)
	outputAddr := to.Seed().Address(0)
	// skip if from peer has no balance
	if addrBalance[inputAddr.String()] == 0 {
		return 0
	}

	// prepare inputs
	resp, err := from.GetUnspentOutputs([]string{inputAddr.String()})
	require.NoErrorf(t, err, "Could not get unspent outputs on %s", from.String())
	value := resp.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	out, err := transaction.OutputIDFromBase58(resp.UnspentOutputs[0].OutputIDs[0].ID)
	require.NoErrorf(t, err, "Invalid unspent outputs ID on %s", from.String())
	inputs := transaction.NewInputs([]transaction.OutputID{out}...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}

	// set balances
	balances := []*balance.Balance{balance.New(balance.ColorIOTA, value)}
	outmap[outputAddr] = balances
	outputs := transaction.NewOutputs(outmap)

	// sign transaction
	txn := transaction.New(inputs, outputs).Sign(sigScheme)

	// send transaction
	_, err = from.SendTransaction(txn.Bytes())
	require.NoErrorf(t, err, "Could not send transaction on %s", from.String())

	return value
}

// CheckBalances performs checks to make sure that all peers have the same ledger state.
func CheckBalances(t *testing.T, peers []*framework.Peer, addrBalance map[string]int64, checkSynchronized bool) {
	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			require.True(t, info.Synced)
		}

		for addr, b := range addrBalance {
			var iotas int64 = 0
			resp, err := peer.GetUnspentOutputs([]string{addr})
			require.NoError(t, err)
			assert.Equal(t, addr, resp.UnspentOutputs[0].Address)

			// calculate the total iotas of an address
			for _, unspents := range resp.UnspentOutputs[0].OutputIDs {
				iotas += unspents.Balances[0].Value
				assert.Equal(t, balance.ColorIOTA.String(), unspents.Balances[0].Color)
			}
			assert.Equal(t, b, iotas)
		}
	}
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n Shutdowner) {
	err := n.Shutdown()
	require.NoError(t, err)
}
