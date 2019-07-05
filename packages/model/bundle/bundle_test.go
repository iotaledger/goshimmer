package bundle

import (
	"testing"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/magiconair/properties/assert"
)

func TestBundle_SettersGetters(t *testing.T) {
	bundleHash := trinary.Trytes("A9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	bundleEssenceHash := trinary.Trytes("B9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	transactions := []trinary.Trytes{
		bundleHash,
		trinary.Trytes("C9999999999999999999999999999999999999999999999999999999999999999999999999999999F"),
	}

	testBundle := New(bundleHash)
	testBundle.SetTransactionHashes(transactions)
	testBundle.SetBundleEssenceHash(bundleEssenceHash)
	testBundle.SetValueBundle(true)

	assert.Equal(t, testBundle.GetHash(), bundleHash, "hash of source")
	assert.Equal(t, testBundle.GetBundleEssenceHash(), bundleEssenceHash, "bundle essence hash of source")
	assert.Equal(t, testBundle.IsValueBundle(), true, "value bundle of source")
	assert.Equal(t, len(testBundle.GetTransactionHashes()), len(transactions), "# of transactions of source")
	assert.Equal(t, testBundle.GetTransactionHashes()[0], transactions[0], "transaction[0] of source")
	assert.Equal(t, testBundle.GetTransactionHashes()[1], transactions[1], "transaction[1] of source")
}

func TestBundle_SettersGettersMarshalUnmarshal(t *testing.T) {
	bundleHash := trinary.Trytes("A9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	bundleEssenceHash := trinary.Trytes("B9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	transactions := []trinary.Trytes{
		bundleHash,
		trinary.Trytes("C9999999999999999999999999999999999999999999999999999999999999999999999999999999F"),
	}

	testBundle := New(bundleHash)
	testBundle.SetTransactionHashes(transactions)
	testBundle.SetBundleEssenceHash(bundleEssenceHash)
	testBundle.SetValueBundle(true)

	var bundleUnmarshaled Bundle
	err := bundleUnmarshaled.Unmarshal(testBundle.Marshal())
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, bundleUnmarshaled.GetHash(), testBundle.GetHash(), "hash of target")
	assert.Equal(t, bundleUnmarshaled.GetBundleEssenceHash(), testBundle.GetBundleEssenceHash(), "bundle essence hash of target")
	assert.Equal(t, bundleUnmarshaled.IsValueBundle(), true, "value bundle of target")
	assert.Equal(t, len(bundleUnmarshaled.GetTransactionHashes()), len(transactions), "# of transactions of target")
	assert.Equal(t, bundleUnmarshaled.GetTransactionHashes()[0], transactions[0], "transaction[0] of target")
	assert.Equal(t, bundleUnmarshaled.GetTransactionHashes()[1], transactions[1], "transaction[1] of target")
}
