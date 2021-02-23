package ledgerstate

import (
	"math/rand"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"
)

func TestED25519Address(t *testing.T) {
	// generate ED25519 public key
	keyPair := ed25519.GenerateKeyPair()
	address := NewED25519Address(keyPair.PublicKey)

	// ED25519 address from bytes
	address1, _, err := ED25519AddressFromBytes(address.Bytes())
	require.NoError(t, err)
	assert.Equal(t, address.Type(), address1.Type())
	assert.Equal(t, address.Digest(), address1.Digest())

	// ED25519 address from bytes using AddressFromBytes
	address2, _, err := AddressFromBytes(address.Bytes())
	require.NoError(t, err)
	assert.Equal(t, address.Type(), address2.Type())
	assert.Equal(t, address.Digest(), address2.Digest())

	// ED25519 address from base58 string
	addressFromBase58, err := AddressFromBase58EncodedString(address.Base58())
	require.NoError(t, err)
	assert.Equal(t, address.Type(), addressFromBase58.Type())
	assert.Equal(t, address.Digest(), addressFromBase58.Digest())
}

func TestBLSAddress(t *testing.T) {
	// generate BLS public key
	suite := bn256.NewSuite()
	rnd := random.New(rand.New(rand.NewSource(42)))
	_, pubKey := bdn.NewKeyPair(suite, rnd)
	pubKeyBytes, err := pubKey.MarshalBinary()
	require.NoError(t, err)

	address := NewBLSAddress(pubKeyBytes)

	// BLS address from bytes
	address1, _, err := BLSAddressFromBytes(address.Bytes())
	require.NoError(t, err)
	assert.Equal(t, address.Type(), address1.Type())
	assert.Equal(t, address.Digest(), address1.Digest())

	// BLS address from bytes using AddressFromBytes
	address2, _, err := AddressFromBytes(address.Bytes())
	require.NoError(t, err)
	assert.Equal(t, address.Type(), address2.Type())
	assert.Equal(t, address.Digest(), address2.Digest())

	// BLS address from base58 string
	addressFromBase58, err := AddressFromBase58EncodedString(address.Base58())
	require.NoError(t, err)
	assert.Equal(t, address.Type(), addressFromBase58.Type())
	assert.Equal(t, address.Digest(), addressFromBase58.Digest())
}
