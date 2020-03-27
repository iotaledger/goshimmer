package signaturescheme

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

var dataToSign = []byte("Hello Boneh-Lynn-Shacham (BLS) --> Boneh-Drijvers-Neven (BDN)")

func TestBLS_base(t *testing.T) {
	blsSigScheme := RandBLS()
	t.Logf("generating random BLS signature scheme: %s\n", blsSigScheme.(*blsSignatureScheme).String())
	signature := blsSigScheme.Sign(dataToSign)

	assert.Equal(t, blsSigScheme.Address(), signature.Address())
	res := signature.IsValid(dataToSign)
	assert.Equal(t, res, true)
}

// number of signatures to aggregate
const numSigs = 100

func TestBLS_aggregation(t *testing.T) {
	sigs := make([]Signature, numSigs)
	sigSchemes := make([]SignatureScheme, numSigs)

	for i := range sigs {
		sigSchemes[i] = RandBLS()
		sigs[i] = sigSchemes[i].Sign(dataToSign)
	}
	aggregatedSig1, err := AggregateBLSSignatures(sigs...)
	assert.Equal(t, err, nil)

	assert.Equal(t, aggregatedSig1.IsValid(dataToSign), true)

	aggregatedScheme, err := AggregateBLSSignatureSchemes(sigSchemes...)
	assert.Equal(t, err, nil)

	if err == nil {
		aggregatedSig2 := aggregatedScheme.Sign(dataToSign)
		assert.Equal(t, aggregatedSig2, aggregatedSig2)
	}
}
