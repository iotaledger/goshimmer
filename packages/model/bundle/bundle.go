package bundle

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/errors"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

type Bundle struct {
	hash               ternary.Trytes
	transactionHashes  []ternary.Trytes
	isValueBundle      bool
	isValueBundleMutex sync.RWMutex
	bundleEssenceHash  ternary.Trytes
	modified           bool
	modifiedMutex      sync.RWMutex
}

func New(headTransactionHash ternary.Trytes) (result *Bundle) {
	result = &Bundle{
		hash: headTransactionHash,
	}

	return
}

func (bundle *Bundle) GetTransactionHashes() []ternary.Trytes {
	return bundle.transactionHashes
}

func (bundle *Bundle) GetHash() ternary.Trytes {
	return bundle.hash
}

func (bundle *Bundle) IsValueBundle() (result bool) {
	bundle.isValueBundleMutex.RLock()
	result = bundle.isValueBundle
	bundle.isValueBundleMutex.RUnlock()

	return
}

func (bundle *Bundle) SetValueBundle(valueBundle bool) {
	bundle.isValueBundleMutex.Lock()
	bundle.isValueBundle = valueBundle
	bundle.isValueBundleMutex.Unlock()
}

func (bundle *Bundle) GetModified() (result bool) {
	bundle.modifiedMutex.RLock()
	result = bundle.modified
	bundle.modifiedMutex.RUnlock()

	return
}

func (bundle *Bundle) SetModified(modified bool) {
	bundle.modifiedMutex.Lock()
	bundle.modified = modified
	bundle.modifiedMutex.Unlock()
}

func (bundle *Bundle) Marshal() []byte {
	return nil
}

func (bundle *Bundle) Unmarshal(data []byte) errors.IdentifiableError {
	return nil
}

func CalculateBundleHash(transactions []*value_transaction.ValueTransaction) ternary.Trytes {
	return (<-Hasher.Hash(transactions[0].GetBundleEssence())).ToTrytes()
}
