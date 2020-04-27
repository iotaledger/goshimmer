package signaturescheme

import (
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/mr-tron/base58"
)

var dataToSign = []byte("Hello Boneh-Lynn-Shacham (BLS) --> Boneh-Drijvers-Neven (BDN)")

func TestBLS_rndSigScheme(t *testing.T) {
	sigScheme := RandBLS()
	t.Logf("generating random BLS signature scheme: %s\n", sigScheme.(*blsSignatureScheme).String())
	signature := sigScheme.Sign(dataToSign)

	assert.Equal(t, sigScheme.Address(), signature.Address())
	res := signature.IsValid(dataToSign)
	assert.Equal(t, res, true)
}

const (
	priKeyTest = "Cjsu52qf28G4oLiUDcimEY7SPbWJQA9zoKCNi4ywMxg"
	pubKeyTest = "28LgNCDp52gTotmd21hcEXKar5tTyxuJKqQdGHCJnZ5Z1M7Rdh4Qo2BYC3s3NicLD99tZ3yX9mZvRmsnQLMRcHnzqgq2CQp7CYWCKfTUT9yzJKUTQ4JmN2DhSkSNc5kau4KE8PRGByQxpiYQq4DRF4Qb3Dn4cHmhTrDi9xQiYTxoAYW"
)

func TestBLS_sigScheme(t *testing.T) {
	priKeyBin, err := base58.Decode(priKeyTest)
	assert.Equal(t, err, nil)

	pubKeyBin, err := base58.Decode(pubKeyTest)
	assert.Equal(t, err, nil)

	sigScheme, err := BLS(priKeyBin, pubKeyBin)
	assert.Equal(t, err, nil)

	signature := sigScheme.Sign(dataToSign)
	assert.Equal(t, sigScheme.Address(), signature.Address())
	assert.Equal(t, signature.IsValid(dataToSign), true)
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
	// aggregate 2 signatures
	a01, err := AggregateBLSSignatures(sigs[0], sigs[1])
	assert.Equal(t, err, nil)
	assert.Equal(t, a01.IsValid(dataToSign), true)

	// aggregate N signatures
	aN, err := AggregateBLSSignatures(sigs...)
	assert.Equal(t, err, nil)
	assert.Equal(t, aN.IsValid(dataToSign), true)
}
