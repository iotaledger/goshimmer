package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
)

func TestSignatures(t *testing.T) {
	dataToSign := []byte("test")

	address1SigScheme := ED25519(ed25119.GenerateKeyPair())
	address2SigScheme := ED25519(ed25119.GenerateKeyPair())

	signatures := New()
	signatures.Add(address1SigScheme.Address(), address1SigScheme.Sign(dataToSign))
	signatures.Add(address2SigScheme.Address(), address2SigScheme.Sign(dataToSign))

	assert.Equal(t, 2, signatures.Size())

	signatures.Add(address1SigScheme.Address(), address1SigScheme.Sign(dataToSign))

	assert.Equal(t, 2, signatures.Size())

	signatures.ForEach(func(address address.Address, signature Signature) bool {
		assert.Equal(t, true, signature.IsValid(dataToSign))

		return true
	})

	clonedSignatures, err, _ := FromBytes(signatures.Bytes())
	if err != nil {
		t.Error(err)

		return
	}

	assert.Equal(t, 2, clonedSignatures.Size())

	clonedSignatures.ForEach(func(address address.Address, signature Signature) bool {
		assert.Equal(t, true, signature.IsValid(dataToSign))

		return true
	})
}
