package value_transaction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/iota.go/trinary"
)

type ValueTransaction struct {
	*meta_transaction.MetaTransaction

	address                       *trinary.Trytes
	addressMutex                  sync.RWMutex
	value                         *int64
	valueMutex                    sync.RWMutex
	timestamp                     *uint
	timestampMutex                sync.RWMutex
	signatureMessageFragment      *trinary.Trytes
	signatureMessageFragmentMutex sync.RWMutex

	trits trinary.Trits
}

func New() (result *ValueTransaction) {
	result = &ValueTransaction{
		MetaTransaction: meta_transaction.New(),
	}

	result.trits = result.MetaTransaction.GetData()

	return
}

func FromMetaTransaction(metaTransaction *meta_transaction.MetaTransaction) *ValueTransaction {
	return &ValueTransaction{
		MetaTransaction: metaTransaction,
		trits:           metaTransaction.GetData(),
	}
}

func FromBytes(bytes []byte) (result *ValueTransaction) {
	trits := trinary.MustBytesToTrits(bytes)
	result = &ValueTransaction{
		MetaTransaction: meta_transaction.FromTrits(trits[:meta_transaction.MARSHALED_TOTAL_SIZE]),
	}

	result.trits = result.MetaTransaction.GetData()

	return
}

// getter for the address (supports concurrency)
func (this *ValueTransaction) GetAddress() (result trinary.Trytes) {
	this.addressMutex.RLock()
	if this.address == nil {
		this.addressMutex.RUnlock()
		this.addressMutex.Lock()
		defer this.addressMutex.Unlock()
		if this.address == nil {
			address := trinary.MustTritsToTrytes(this.trits[ADDRESS_OFFSET:ADDRESS_END])

			this.address = &address
		}
	} else {
		defer this.addressMutex.RUnlock()
	}

	result = *this.address

	return
}

// setter for the address (supports concurrency)
func (this *ValueTransaction) SetAddress(address trinary.Trytes) bool {
	this.addressMutex.RLock()
	if this.address == nil || *this.address != address {
		this.addressMutex.RUnlock()
		this.addressMutex.Lock()
		defer this.addressMutex.Unlock()
		if this.address == nil || *this.address != address {
			this.address = &address

			this.BlockHasher()
			copy(this.trits[ADDRESS_OFFSET:ADDRESS_END], trinary.MustTrytesToTrits(address)[:ADDRESS_SIZE])
			this.UnblockHasher()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.addressMutex.RUnlock()
	}

	return false
}

// getter for the value (supports concurrency)
func (this *ValueTransaction) GetValue() (result int64) {
	this.valueMutex.RLock()
	if this.value == nil {
		this.valueMutex.RUnlock()
		this.valueMutex.Lock()
		defer this.valueMutex.Unlock()
		if this.value == nil {
			value := trinary.TritsToInt(this.trits[VALUE_OFFSET:VALUE_END])

			this.value = &value
		}
	} else {
		defer this.valueMutex.RUnlock()
	}

	result = *this.value

	return
}

// setter for the value (supports concurrency)
func (this *ValueTransaction) SetValue(value int64) bool {
	this.valueMutex.RLock()
	if this.value == nil || *this.value != value {
		this.valueMutex.RUnlock()
		this.valueMutex.Lock()
		defer this.valueMutex.Unlock()
		if this.value == nil || *this.value != value {
			this.value = &value

			this.BlockHasher()
			copy(this.trits[VALUE_OFFSET:], trinary.IntToTrits(value))
			this.UnblockHasher()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.valueMutex.RUnlock()
	}

	return false
}

// getter for the timestamp (supports concurrency)
func (this *ValueTransaction) GetTimestamp() (result uint) {
	this.timestampMutex.RLock()
	if this.timestamp == nil {
		this.timestampMutex.RUnlock()
		this.timestampMutex.Lock()
		defer this.timestampMutex.Unlock()
		if this.timestamp == nil {
			timestamp := uint(trinary.TritsToInt(this.trits[TIMESTAMP_OFFSET:TIMESTAMP_END]))

			this.timestamp = &timestamp
		}
	} else {
		defer this.timestampMutex.RUnlock()
	}

	result = *this.timestamp

	return
}

// setter for the timestamp (supports concurrency)
func (this *ValueTransaction) SetTimestamp(timestamp uint) bool {
	this.timestampMutex.RLock()
	if this.timestamp == nil || *this.timestamp != timestamp {
		this.timestampMutex.RUnlock()
		this.timestampMutex.Lock()
		defer this.timestampMutex.Unlock()
		if this.timestamp == nil || *this.timestamp != timestamp {
			this.timestamp = &timestamp

			this.BlockHasher()
			copy(this.trits[TIMESTAMP_OFFSET:TIMESTAMP_END], trinary.MustPadTrits(trinary.IntToTrits(int64(timestamp)), TIMESTAMP_SIZE)[:TIMESTAMP_SIZE])
			this.UnblockHasher()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.timestampMutex.RUnlock()
	}

	return false
}

func (this *ValueTransaction) GetBundleEssence(includeSignatureMessageFragment bool) (result trinary.Trits) {
	this.signatureMessageFragmentMutex.RLock()

	result = make(trinary.Trits, BUNDLE_ESSENCE_SIZE)

	this.addressMutex.RLock()
	copy(result[0:], this.trits[ADDRESS_OFFSET:VALUE_END])
	this.addressMutex.RUnlock()

	if includeSignatureMessageFragment {
		copy(result[VALUE_END:], this.trits[SIGNATURE_MESSAGE_FRAGMENT_OFFSET:SIGNATURE_MESSAGE_FRAGMENT_END])
	}

	this.signatureMessageFragmentMutex.RUnlock()

	return
}

// getter for the signatureMessageFragmetn (supports concurrency)
func (this *ValueTransaction) GetSignatureMessageFragment() (result trinary.Trytes) {
	this.signatureMessageFragmentMutex.RLock()
	if this.signatureMessageFragment == nil {
		this.signatureMessageFragmentMutex.RUnlock()
		this.signatureMessageFragmentMutex.Lock()
		defer this.signatureMessageFragmentMutex.Unlock()
		if this.signatureMessageFragment == nil {
			signatureMessageFragment := trinary.MustTritsToTrytes(this.trits[SIGNATURE_MESSAGE_FRAGMENT_OFFSET:SIGNATURE_MESSAGE_FRAGMENT_END])

			this.signatureMessageFragment = &signatureMessageFragment
		}
	} else {
		defer this.signatureMessageFragmentMutex.RUnlock()
	}

	result = *this.signatureMessageFragment

	return
}

// setter for the nonce (supports concurrency)
func (this *ValueTransaction) SetSignatureMessageFragment(signatureMessageFragment trinary.Trytes) bool {
	this.signatureMessageFragmentMutex.RLock()
	if this.signatureMessageFragment == nil || *this.signatureMessageFragment != signatureMessageFragment {
		this.signatureMessageFragmentMutex.RUnlock()
		this.signatureMessageFragmentMutex.Lock()
		defer this.signatureMessageFragmentMutex.Unlock()
		if this.signatureMessageFragment == nil || *this.signatureMessageFragment != signatureMessageFragment {
			this.signatureMessageFragment = &signatureMessageFragment

			this.BlockHasher()
			copy(this.trits[SIGNATURE_MESSAGE_FRAGMENT_OFFSET:SIGNATURE_MESSAGE_FRAGMENT_END], trinary.MustTrytesToTrits(signatureMessageFragment)[:SIGNATURE_MESSAGE_FRAGMENT_SIZE])
			this.UnblockHasher()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.signatureMessageFragmentMutex.RUnlock()
	}

	return false
}
