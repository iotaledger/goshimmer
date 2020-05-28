package transaction

import (
	"bytes"
	"strings"
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

func TestTooLongDataPayload(t *testing.T) {
	sigScheme := signaturescheme.ED25519(ed25519.GenerateKeyPair())
	addr := sigScheme.Address()
	o1 := NewOutputID(addr, RandomID())
	inputs := NewInputs(o1)
	bal := balance.New(balance.ColorIOTA, 1)
	outputs := NewOutputs(map[address.Address][]*balance.Balance{addr: {bal}})
	tx := New(inputs, outputs)

	dataPayload := []byte(strings.Repeat("1", MaxDataPayloadSize+1))
	err := tx.SetDataPayload(dataPayload)
	assert.Error(t, err)
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

	tx1 := Transaction{}
	_, err := tx1.UnmarshalObjectStorageValue(v)
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

	tx1 := Transaction{}
	_, err = tx1.UnmarshalObjectStorageValue(v)

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
