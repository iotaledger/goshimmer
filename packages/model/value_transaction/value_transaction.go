package value_transaction

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

type ValueTransaction struct {
	*meta_transaction.MetaTransaction

	address                       *ternary.Trinary
	addressMutex                  sync.RWMutex
	value                         *int64
	valueMutex                    sync.RWMutex
	timestamp                     *uint
	timestampMutex                sync.RWMutex
	nonce                         *ternary.Trinary
	nonceMutex                    sync.RWMutex
	signatureMessageFragment      *ternary.Trinary
	signatureMessageFragmentMutex sync.RWMutex

	trits ternary.Trits
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
		trits: metaTransaction.GetData(),
	}
}

func FromBytes(bytes []byte) (result *ValueTransaction) {
	result = &ValueTransaction{
		MetaTransaction: meta_transaction.FromTrits(ternary.BytesToTrits(bytes)[:meta_transaction.MARSHALLED_TOTAL_SIZE]),
	}

	result.trits = result.MetaTransaction.GetData()

	return
}

// getter for the address (supports concurrency)
func (this *ValueTransaction) GetAddress() (result ternary.Trinary) {
	this.addressMutex.RLock()
	if this.address == nil {
		this.addressMutex.RUnlock()
		this.addressMutex.Lock()
		defer this.addressMutex.Unlock()
		if this.address == nil {
			address := this.trits[ADDRESS_OFFSET:ADDRESS_END].ToTrinary()

			this.address = &address
		}
	} else {
		defer this.addressMutex.RUnlock()
	}

	result = *this.address

	return
}

// setter for the address (supports concurrency)
func (this *ValueTransaction) SetAddress(address ternary.Trinary) bool {
	this.addressMutex.RLock()
	if this.address == nil || *this.address != address {
		this.addressMutex.RUnlock()
		this.addressMutex.Lock()
		defer this.addressMutex.Unlock()
		if this.address == nil || *this.address != address {
			this.address = &address

			this.BlockHasher()
			copy(this.trits[ADDRESS_OFFSET:ADDRESS_END], address.ToTrits()[:ADDRESS_SIZE])
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
			value := this.trits[VALUE_OFFSET:VALUE_END].ToInt64()

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
			copy(this.trits[VALUE_OFFSET:VALUE_END], ternary.Int64ToTrits(value)[:VALUE_SIZE])
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
			timestamp := this.trits[TIMESTAMP_OFFSET:TIMESTAMP_END].ToUint()

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
			copy(this.trits[TIMESTAMP_OFFSET:TIMESTAMP_END], ternary.UintToTrits(timestamp)[:TIMESTAMP_SIZE])
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

// getter for the nonce (supports concurrency)
func (this *ValueTransaction) GetNonce() (result ternary.Trinary) {
	this.nonceMutex.RLock()
	if this.nonce == nil {
		this.nonceMutex.RUnlock()
		this.nonceMutex.Lock()
		defer this.nonceMutex.Unlock()
		if this.nonce == nil {
			nonce := this.trits[NONCE_OFFSET:NONCE_END].ToTrinary()

			this.nonce = &nonce
		}
	} else {
		defer this.nonceMutex.RUnlock()
	}

	result = *this.nonce

	return
}

// setter for the nonce (supports concurrency)
func (this *ValueTransaction) SetNonce(nonce ternary.Trinary) bool {
	this.nonceMutex.RLock()
	if this.nonce == nil || *this.nonce != nonce {
		this.nonceMutex.RUnlock()
		this.nonceMutex.Lock()
		defer this.nonceMutex.Unlock()
		if this.nonce == nil || *this.nonce != nonce {
			this.nonce = &nonce

			this.BlockHasher()
			copy(this.trits[NONCE_OFFSET:NONCE_END], nonce.ToTrits()[:NONCE_SIZE])
			this.UnblockHasher()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.nonceMutex.RUnlock()
	}

	return false
}

// getter for the signatureMessageFragmetn (supports concurrency)
func (this *ValueTransaction) GetSignatureMessageFragment() (result ternary.Trinary) {
	this.signatureMessageFragmentMutex.RLock()
	if this.signatureMessageFragment == nil {
		this.signatureMessageFragmentMutex.RUnlock()
		this.signatureMessageFragmentMutex.Lock()
		defer this.signatureMessageFragmentMutex.Unlock()
		if this.signatureMessageFragment == nil {
			signatureMessageFragment := this.trits[SIGNATURE_MESSAGE_FRAGMENT_OFFSET:SIGNATURE_MESSAGE_FRAGMENT_END].ToTrinary()

			this.signatureMessageFragment = &signatureMessageFragment
		}
	} else {
		defer this.signatureMessageFragmentMutex.RUnlock()
	}

	result = *this.signatureMessageFragment

	return
}

// setter for the nonce (supports concurrency)
func (this *ValueTransaction) SetSignatureMessageFragment(signatureMessageFragment ternary.Trinary) bool {
	this.signatureMessageFragmentMutex.RLock()
	if this.signatureMessageFragment == nil || *this.signatureMessageFragment != signatureMessageFragment {
		this.signatureMessageFragmentMutex.RUnlock()
		this.signatureMessageFragmentMutex.Lock()
		defer this.signatureMessageFragmentMutex.Unlock()
		if this.signatureMessageFragment == nil || *this.signatureMessageFragment != signatureMessageFragment {
			this.signatureMessageFragment = &signatureMessageFragment

			this.BlockHasher()
			copy(this.trits[SIGNATURE_MESSAGE_FRAGMENT_OFFSET:SIGNATURE_MESSAGE_FRAGMENT_END], signatureMessageFragment.ToTrits()[:SIGNATURE_MESSAGE_FRAGMENT_SIZE])
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
