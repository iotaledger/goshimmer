package value

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransactionFromJSON(t *testing.T) {
	// generate ed25519 keypair
	keyPair1 := ed25519.GenerateKeyPair()
	sigScheme1 := signaturescheme.ED25519(keyPair1)
	// generate BLS keypair
	sigScheme2 := signaturescheme.RandBLS()
	addr1 := sigScheme1.Address()
	addr2 := sigScheme2.Address()
	// create first input
	o1 := transaction.NewOutputID(addr1, transaction.RandomID())
	// create second input
	o2 := transaction.NewOutputID(addr2, transaction.RandomID())
	inputs := transaction.NewInputs(o1, o2)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{address.Random(): {bal}})
	tx := transaction.New(inputs, outputs)
	tx.SetDataPayload([]byte("TEST"))
	// sign with ed25519
	tx.Sign(sigScheme1)
	// sign with BLS
	tx.Sign(sigScheme2)

	// Parse inputs to base58
	inputsBase58 := []string{}
	inputs.ForEach(func(outputId transaction.OutputID) bool {
		inputsBase58 = append(inputsBase58, base58.Encode(outputId.Bytes()))
		return true
	})

	// Parse outputs to base58
	outputsBase58 := []Output{}
	outputs.ForEach(func(address address.Address, balances []*balance.Balance) bool {
		var b []Balance
		for _, balance := range balances {
			b = append(b, Balance{
				Value: balance.Value,
				Color: balance.Color.String(),
			})
		}
		t := Output{
			Address:  address.String(),
			Balances: b,
		}
		outputsBase58 = append(outputsBase58, t)

		return true
	})

	// Parse signatures to base58
	signaturesBase58 := []Signature{}
	for _, signature := range tx.Signatures() {
		signaturesBase58 = append(signaturesBase58, Signature{
			Version:   signature.Bytes()[0],
			PublicKey: base58.Encode(signature.Bytes()[1 : signature.PublicKeySize()+1]),
			Signature: base58.Encode(signature.Bytes()[1+signature.PublicKeySize():]),
		})
	}

	// create tx JSON
	jsonRequest := SendTransactionByJSONRequest{
		Inputs:     inputsBase58,
		Outputs:    outputsBase58,
		Data:       []byte("TEST"),
		Signatures: signaturesBase58,
	}
	txFromJSON, err := NewTransactionFromJSON(jsonRequest)
	require.NoError(t, err)

	// compare signatures
	assert.Equal(t, tx.SignatureBytes(), txFromJSON.SignatureBytes())

	// conmpare transactions
	assert.Equal(t, tx, txFromJSON)
}
