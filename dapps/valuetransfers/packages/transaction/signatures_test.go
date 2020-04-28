package transaction

import (
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
)

func TestSignatures(t *testing.T) {
	dataToSign := []byte("test")

	address1SigScheme := signaturescheme.ED25519(ed25519.GenerateKeyPair())
	address2SigScheme := signaturescheme.ED25519(ed25519.GenerateKeyPair())
	address3SigScheme := signaturescheme.RandBLS()

	signatures := NewSignatures()
	signatures.Add(address1SigScheme.Address(), address1SigScheme.Sign(dataToSign))
	signatures.Add(address2SigScheme.Address(), address2SigScheme.Sign(dataToSign))
	signatures.Add(address3SigScheme.Address(), address3SigScheme.Sign(dataToSign))

	assert.Equal(t, 3, signatures.Size())

	signatures.Add(address1SigScheme.Address(), address1SigScheme.Sign(dataToSign))

	assert.Equal(t, 3, signatures.Size())

	signatures.ForEach(func(address address.Address, signature signaturescheme.Signature) bool {
		assert.Equal(t, true, signature.IsValid(dataToSign))

		return true
	})

	clonedSignatures, err, _ := SignaturesFromBytes(signatures.Bytes())
	if err != nil {
		t.Error(err)

		return
	}

	assert.Equal(t, 3, clonedSignatures.Size())

	clonedSignatures.ForEach(func(address address.Address, signature signaturescheme.Signature) bool {
		assert.Equal(t, true, signature.IsValid(dataToSign))

		return true
	})
}
