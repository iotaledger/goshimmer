package transaction

import (
	"bytes"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
)

func TestEmptyDataPayload(t *testing.T) {
	sigScheme := signaturescheme.ED25519(ed25519.GenerateKeyPair())
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)
	tx.Sign(sigScheme)
	check := tx.SignaturesValid()

	assert.Equal(t, true, check)
}

func TestEmptyDataPayloadString(t *testing.T) {
	sigScheme := signaturescheme.ED25519(ed25519.GenerateKeyPair())
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)
	tx.Sign(sigScheme)
	check := tx.SignaturesValid()

	assert.True(t, check)

	t.Logf("%s", tx.String())
}

func TestShortDataPayload(t *testing.T) {
	sigScheme := signaturescheme.ED25519(ed25519.GenerateKeyPair())
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)

	dataPayload := []byte("data payload test")
	err := tx.SetDataPayload(dataPayload)
	assert.NoError(t, err)

	dpBack := tx.GetDataPayload()
	assert.Equal(t, true, bytes.Equal(dpBack, dataPayload))

	tx.Sign(sigScheme)
	check := tx.SignaturesValid()
	assert.Equal(t, true, check)

	// corrupt data payload bytes
	// reset essence to force recalculation
	tx.essenceBytes = nil
	dataPayload[2] = '?'
	err = tx.SetDataPayload(dataPayload)
	assert.NoError(t, err)

	// expect signature is not valid
	check = tx.SignaturesValid()
	assert.Equal(t, false, check)
}

func TestMarshalingEmptyDataPayload(t *testing.T) {
	sigScheme := signaturescheme.RandBLS()
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)

	tx.Sign(sigScheme)
	check := tx.SignaturesValid()
	assert.Equal(t, true, check)

	v := tx.ObjectStorageValue()

	tx1, _, err := FromBytes(v)
	assert.NoError(t, err)

	assert.Equal(t, true, tx1.SignaturesValid())
	assert.Equal(t, true, bytes.Equal(tx1.ID().Bytes(), tx.ID().Bytes()))
}

func TestMarshalingDataPayload(t *testing.T) {
	sigScheme := signaturescheme.RandBLS()
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)

	dataPayload := []byte("data payload test")
	err := tx.SetDataPayload(dataPayload)
	assert.NoError(t, err)

	tx.Sign(sigScheme)
	check := tx.SignaturesValid()
	assert.Equal(t, true, check)

	v := tx.ObjectStorageValue()

	tx1, _, err := FromBytes(v)

	assert.NoError(t, err)
	assert.Equal(t, true, tx1.SignaturesValid())

	assert.Equal(t, true, bytes.Equal(tx1.ID().Bytes(), tx.ID().Bytes()))
}

func TestPutSignatureValid(t *testing.T) {
	sigScheme := signaturescheme.RandBLS()
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)

	dataPayload := []byte("data payload test")
	err := tx.SetDataPayload(dataPayload)
	assert.NoError(t, err)

	signature := sigScheme.Sign(tx.EssenceBytes())
	assert.Equal(t, signature.IsValid(tx.EssenceBytes()), true)

	err = tx.PutSignature(signature)
	assert.NoError(t, err)

	check := tx.SignaturesValid()
	assert.Equal(t, true, check)
}

func TestPutSignatureInvalid(t *testing.T) {
	sigScheme := signaturescheme.RandBLS()
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)

	dataPayload := []byte("data payload test")
	err := tx.SetDataPayload(dataPayload)
	assert.NoError(t, err)

	signatureValid := sigScheme.Sign(tx.EssenceBytes())
	assert.Equal(t, true, signatureValid.IsValid(tx.EssenceBytes()))

	sigBytes := make([]byte, len(signatureValid.Bytes()))
	copy(sigBytes, signatureValid.Bytes())
	// inverse last byte --> corrupt the signatureValid
	sigBytes[len(sigBytes)-1] = ^sigBytes[len(sigBytes)-1]

	sigCorrupted, consumed, err := signaturescheme.BLSSignatureFromBytes(sigBytes)

	assert.NoError(t, err)
	assert.Equal(t, consumed, len(sigBytes))
	assert.Equal(t, false, sigCorrupted.IsValid(tx.EssenceBytes()))

	err = tx.PutSignature(sigCorrupted)
	// error expected
	assert.Error(t, err)

	// 0 signatures is not valid
	assert.Equal(t, true, !tx.SignaturesValid())

	err = tx.PutSignature(signatureValid)
	// no error expected
	assert.NoError(t, err)

	// valid signatures expected
	assert.Equal(t, true, tx.SignaturesValid())
}

func TestInputCounts(t *testing.T) {
	tx1 := createTransaction(MaxTransactionInputCount+1, 1)
	assert.False(t, tx1.InputsCountValid())

	tx2 := createTransaction(MaxTransactionInputCount-1, 1)
	assert.True(t, tx2.InputsCountValid())
}

func createTransaction(inputCount int, outputCount int) *Transaction {
	outputIds := make([]OutputID, 0)
	for i := 0; i < inputCount; i++ {
		outputIds = append(outputIds, NewOutputID(address.Random(), RandomID()))
	}
	inputs := NewInputs(outputIds...)

	bal := balance.New(balance.ColorIOTA, 1)
	outputMap := make(map[address.Address][]*balance.Balance)
	for i := 0; i < outputCount; i++ {
		outputMap[address.Random()] = []*balance.Balance{bal}
	}
	outputs := NewOutputs(outputMap)

	return New(inputs, outputs)
}
