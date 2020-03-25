package signaturescheme

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestBLS_BDN(t *testing.T) {
	dataToSign := []byte("testtesttest")

	blsSigScheme := RandBLS_BDN()
	signature := blsSigScheme.Sign(dataToSign)

	assert.Equal(t, blsSigScheme.Address() == signature.Address(), true)
	assert.Equal(t, signature.IsValid(dataToSign), true)
}
