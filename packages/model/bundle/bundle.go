package bundle

import (
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"

	"github.com/iotaledger/goshimmer/packages/model"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

type Bundle struct {
	hash                   trinary.Trytes
	hashMutex              sync.RWMutex
	transactionHashes      []trinary.Trytes
	transactionHashesMutex sync.RWMutex
	isValueBundle          bool
	isValueBundleMutex     sync.RWMutex
	bundleEssenceHash      trinary.Trytes
	bundleEssenceHashMutex sync.RWMutex
	modified               bool
	modifiedMutex          sync.RWMutex
}

func New(headTransactionHash trinary.Trytes) (result *Bundle) {
	result = &Bundle{
		hash: headTransactionHash,
	}

	return
}

func (bundle *Bundle) GetHash() (result trinary.Trytes) {
	bundle.hashMutex.RLock()
	result = bundle.hash
	bundle.hashMutex.RUnlock()

	return
}

func (bundle *Bundle) SetHash(hash trinary.Trytes) {
	bundle.hashMutex.Lock()
	bundle.hash = hash
	bundle.hashMutex.Unlock()
}

func (bundle *Bundle) GetTransactionHashes() (result []trinary.Trytes) {
	bundle.transactionHashesMutex.RLock()
	result = bundle.transactionHashes
	bundle.transactionHashesMutex.RUnlock()

	return
}

func (bundle *Bundle) SetTransactionHashes(transactionHashes []trinary.Trytes) {
	bundle.transactionHashesMutex.Lock()
	bundle.transactionHashes = transactionHashes
	bundle.transactionHashesMutex.Unlock()
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

func (bundle *Bundle) GetBundleEssenceHash() (result trinary.Trytes) {
	bundle.bundleEssenceHashMutex.RLock()
	result = bundle.bundleEssenceHash
	bundle.bundleEssenceHashMutex.RUnlock()

	return
}

func (bundle *Bundle) SetBundleEssenceHash(bundleEssenceHash trinary.Trytes) {
	bundle.bundleEssenceHashMutex.Lock()
	bundle.bundleEssenceHash = bundleEssenceHash
	bundle.bundleEssenceHashMutex.Unlock()
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

func (bundle *Bundle) Marshal() (result []byte) {
	bundle.hashMutex.RLock()
	bundle.bundleEssenceHashMutex.RLock()
	bundle.isValueBundleMutex.RLock()
	bundle.transactionHashesMutex.RLock()

	result = make([]byte, MARSHALED_MIN_SIZE+len(bundle.transactionHashes)*MARSHALED_TRANSACTION_HASH_SIZE)

	binary.BigEndian.PutUint64(result[MARSHALED_TRANSACTIONS_COUNT_START:MARSHALED_TRANSACTIONS_COUNT_END], uint64(len(bundle.transactionHashes)))

	copy(result[MARSHALED_HASH_START:MARSHALED_HASH_END], typeutils.StringToBytes(bundle.hash))
	copy(result[MARSHALED_BUNDLE_ESSENCE_HASH_START:MARSHALED_BUNDLE_ESSENCE_HASH_END], typeutils.StringToBytes(bundle.bundleEssenceHash))

	var flags bitmask.BitMask
	if bundle.isValueBundle {
		flags = flags.SetFlag(0)
	}
	result[MARSHALED_FLAGS_START] = *(*byte)(unsafe.Pointer(&flags))

	i := 0
	for _, hash := range bundle.transactionHashes {
		var HASH_START = MARSHALED_APPROVERS_HASHES_START + i*(MARSHALED_TRANSACTION_HASH_SIZE)
		var HASH_END = HASH_START + MARSHALED_TRANSACTION_HASH_SIZE

		copy(result[HASH_START:HASH_END], typeutils.StringToBytes(hash))

		i++
	}

	bundle.transactionHashesMutex.RUnlock()
	bundle.isValueBundleMutex.RUnlock()
	bundle.bundleEssenceHashMutex.RUnlock()
	bundle.hashMutex.RUnlock()

	return
}

func (bundle *Bundle) Unmarshal(data []byte) error {
	dataLen := len(data)

	if dataLen < MARSHALED_MIN_SIZE {
		return fmt.Errorf("%w: marshaled bundle is too short", model.ErrMarshalFailed)
	}

	hashesCount := binary.BigEndian.Uint64(data[MARSHALED_TRANSACTIONS_COUNT_START:MARSHALED_TRANSACTIONS_COUNT_END])

	if dataLen < MARSHALED_MIN_SIZE+int(hashesCount)*MARSHALED_TRANSACTION_HASH_SIZE {
		return fmt.Errorf("%w: marshaled bundle is too short for %d transactions", model.ErrMarshalFailed, hashesCount)
	}

	bundle.hashMutex.Lock()
	bundle.bundleEssenceHashMutex.Lock()
	bundle.isValueBundleMutex.Lock()
	bundle.transactionHashesMutex.Lock()

	bundle.hash = trinary.Trytes(typeutils.BytesToString(data[MARSHALED_HASH_START:MARSHALED_HASH_END]))
	bundle.bundleEssenceHash = trinary.Trytes(typeutils.BytesToString(data[MARSHALED_BUNDLE_ESSENCE_HASH_START:MARSHALED_BUNDLE_ESSENCE_HASH_END]))

	flags := bitmask.BitMask(data[MARSHALED_FLAGS_START])
	if flags.HasFlag(0) {
		bundle.isValueBundle = true
	}

	bundle.transactionHashes = make([]trinary.Trytes, hashesCount)
	for i := uint64(0); i < hashesCount; i++ {
		var HASH_START = MARSHALED_APPROVERS_HASHES_START + i*(MARSHALED_TRANSACTION_HASH_SIZE)
		var HASH_END = HASH_START + MARSHALED_TRANSACTION_HASH_SIZE

		bundle.transactionHashes[i] = trinary.Trytes(typeutils.BytesToString(data[HASH_START:HASH_END]))
	}

	bundle.transactionHashesMutex.Unlock()
	bundle.isValueBundleMutex.Unlock()
	bundle.bundleEssenceHashMutex.Unlock()
	bundle.hashMutex.Unlock()

	return nil
}
