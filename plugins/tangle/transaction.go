package tangle

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
)

// region type definition and constructor //////////////////////////////////////////////////////////////////////////////

type Transaction struct {
	// wrapped objects
	rawTransaction   *transaction.Transaction
	rawMetaData      *TransactionMetadata
	rawMetaDataMutex sync.RWMutex

	// mapped raw transaction properties
	hash                          *ternary.Trinary
	hashMutex                     sync.RWMutex
	signatureMessageFragment      *ternary.Trinary
	signatureMessageFragmentMutex sync.RWMutex
	address                       *ternary.Trinary
	addressMutex                  sync.RWMutex
	value                         *int64
	valueMutex                    sync.RWMutex
	timestamp                     *uint64
	timestampMutex                sync.RWMutex
	currentIndex                  *uint64
	currentIndexMutex             sync.RWMutex
	latestIndex                   *uint64
	latestIndexMutex              sync.RWMutex
	bundleHash                    *ternary.Trinary
	bundleHashMutex               sync.RWMutex
	trunkTransactionHash          *ternary.Trinary
	trunkTransactionHashMutex     sync.RWMutex
	branchTransactionHash         *ternary.Trinary
	branchTransactionHashMutex    sync.RWMutex
	tag                           *ternary.Trinary
	tagMutex                      sync.RWMutex
	nonce                         *ternary.Trinary
	nonceMutex                    sync.RWMutex

	// additional runtime specific metadata
	modified      bool
	modifiedMutex sync.RWMutex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region getters and setters //////////////////////////////////////////////////////////////////////////////////////////

func (transaction *Transaction) GetMetaData() (*TransactionMetadata, errors.IdentifiableError) {
	transaction.rawMetaDataMutex.RLock()
	if transaction.rawMetaData == nil {
		transaction.rawMetaDataMutex.RUnlock()
		transaction.rawMetaDataMutex.Lock()
		defer transaction.rawMetaDataMutex.Unlock()
		if transaction.rawMetaData == nil {
			if metaData, err := getTransactionMetadataFromDatabase(transaction.GetHash()); err != nil {
				return nil, err
			} else if metaData == nil {
				transaction.rawMetaData = NewTransactionMetadata(transaction.GetHash())
			} else {
				transaction.rawMetaData = metaData
			}

			return transaction.rawMetaData, nil
		}
	}

	defer transaction.rawMetaDataMutex.RUnlock()

	return transaction.rawMetaData, nil
}

// getter for the hash (supports concurrency)
func (transaction *Transaction) GetHash() ternary.Trinary {
	transaction.hashMutex.RLock()
	if transaction.hash == nil {
		transaction.hashMutex.RUnlock()
		transaction.hashMutex.Lock()
		defer transaction.hashMutex.Unlock()
		if transaction.hash == nil {
			return transaction.parseHash()
		}
	}

	defer transaction.hashMutex.RUnlock()

	return *transaction.hash
}

// setter for the hash (supports concurrency)
func (transaction *Transaction) SetHash(hash ternary.Trinary) {
	transaction.hashMutex.RLock()
	if transaction.hash == nil || *transaction.hash != hash {
		transaction.hashMutex.RUnlock()
		transaction.hashMutex.Lock()
		defer transaction.hashMutex.Unlock()
		if transaction.hash == nil || *transaction.hash != hash {
			*transaction.hash = hash

			transaction.SetModified(true)
		}
	} else {
		transaction.hashMutex.RUnlock()
	}
}

// restores the hash from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseHash() ternary.Trinary {
	transaction.hashMutex.Lock()
	defer transaction.hashMutex.Unlock()

	return transaction.parseHash()
}

// parses the hash from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseHash() ternary.Trinary {
	*transaction.hash = transaction.rawTransaction.Hash.ToTrinary()

	return *transaction.hash
}

// getter for the address (supports concurrency)
func (transaction *Transaction) GetAddress() ternary.Trinary {
	transaction.addressMutex.RLock()
	if transaction.address == nil {
		transaction.addressMutex.RUnlock()
		transaction.addressMutex.Lock()
		defer transaction.addressMutex.Unlock()
		if transaction.address == nil {
			return transaction.parseAddress()
		}
	}

	defer transaction.addressMutex.RUnlock()

	return *transaction.address
}

// setter for the address (supports concurrency)
func (transaction *Transaction) SetAddress(address ternary.Trinary) {
	transaction.addressMutex.RLock()
	if transaction.address == nil || *transaction.address != address {
		transaction.addressMutex.RUnlock()
		transaction.addressMutex.Lock()
		defer transaction.addressMutex.Unlock()
		if transaction.address == nil || *transaction.address != address {
			*transaction.address = address

			transaction.SetModified(true)
		}
	} else {
		transaction.addressMutex.RUnlock()
	}
}

// restores the address from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseAddress() ternary.Trinary {
	transaction.addressMutex.Lock()
	defer transaction.addressMutex.Unlock()

	return transaction.parseAddress()
}

// parses the address from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseAddress() ternary.Trinary {
	*transaction.address = transaction.rawTransaction.Hash.ToTrinary()

	return *transaction.address
}

// getter for the value (supports concurrency)
func (transaction *Transaction) GetValue() int64 {
	transaction.valueMutex.RLock()
	if transaction.value == nil {
		transaction.valueMutex.RUnlock()
		transaction.valueMutex.Lock()
		defer transaction.valueMutex.Unlock()
		if transaction.value == nil {
			return transaction.parseValue()
		}
	}

	defer transaction.valueMutex.RUnlock()

	return *transaction.value
}

// setter for the value (supports concurrency)
func (transaction *Transaction) SetValue(value int64) {
	transaction.valueMutex.RLock()
	if transaction.value == nil || *transaction.value != value {
		transaction.valueMutex.RUnlock()
		transaction.valueMutex.Lock()
		defer transaction.valueMutex.Unlock()
		if transaction.value == nil || *transaction.value != value {
			*transaction.value = value

			transaction.SetModified(true)
		}
	} else {
		transaction.valueMutex.RUnlock()
	}
}

// restores the value from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseValue() int64 {
	transaction.valueMutex.Lock()
	defer transaction.valueMutex.Unlock()

	return transaction.parseValue()
}

// parses the value from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseValue() int64 {
	*transaction.value = transaction.rawTransaction.Value.ToInt64()

	return *transaction.value
}

// getter for the timestamp (supports concurrency)
func (transaction *Transaction) GetTimestamp() uint64 {
	transaction.timestampMutex.RLock()
	if transaction.timestamp == nil {
		transaction.timestampMutex.RUnlock()
		transaction.timestampMutex.Lock()
		defer transaction.timestampMutex.Unlock()
		if transaction.timestamp == nil {
			return transaction.parseTimestamp()
		}
	}

	defer transaction.timestampMutex.RUnlock()

	return *transaction.timestamp
}

// setter for the timestamp (supports concurrency)
func (transaction *Transaction) SetTimestamp(timestamp uint64) {
	transaction.timestampMutex.RLock()
	if transaction.timestamp == nil || *transaction.timestamp != timestamp {
		transaction.timestampMutex.RUnlock()
		transaction.timestampMutex.Lock()
		defer transaction.timestampMutex.Unlock()
		if transaction.timestamp == nil || *transaction.timestamp != timestamp {
			*transaction.timestamp = timestamp

			transaction.SetModified(true)
		}
	} else {
		transaction.timestampMutex.RUnlock()
	}
}

// restores the timestamp from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseTimestamp() uint64 {
	transaction.timestampMutex.Lock()
	defer transaction.timestampMutex.Unlock()

	return transaction.parseTimestamp()
}

// parses the timestamp from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseTimestamp() uint64 {
	*transaction.timestamp = transaction.rawTransaction.Timestamp.ToUint64()

	return *transaction.timestamp
}

// getter for the currentIndex (supports concurrency)
func (transaction *Transaction) GetCurrentIndex() uint64 {
	transaction.currentIndexMutex.RLock()
	if transaction.currentIndex == nil {
		transaction.currentIndexMutex.RUnlock()
		transaction.currentIndexMutex.Lock()
		defer transaction.currentIndexMutex.Unlock()
		if transaction.currentIndex == nil {
			return transaction.parseCurrentIndex()
		}
	}

	defer transaction.currentIndexMutex.RUnlock()

	return *transaction.currentIndex
}

// setter for the currentIndex (supports concurrency)
func (transaction *Transaction) SetCurrentIndex(currentIndex uint64) {
	transaction.currentIndexMutex.RLock()
	if transaction.currentIndex == nil || *transaction.currentIndex != currentIndex {
		transaction.currentIndexMutex.RUnlock()
		transaction.currentIndexMutex.Lock()
		defer transaction.currentIndexMutex.Unlock()
		if transaction.currentIndex == nil || *transaction.currentIndex != currentIndex {
			*transaction.currentIndex = currentIndex

			transaction.SetModified(true)
		}
	} else {
		transaction.currentIndexMutex.RUnlock()
	}
}

// restores the currentIndex from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseCurrentIndex() uint64 {
	transaction.currentIndexMutex.Lock()
	defer transaction.currentIndexMutex.Unlock()

	return transaction.parseCurrentIndex()
}

// parses the currentIndex from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseCurrentIndex() uint64 {
	*transaction.currentIndex = transaction.rawTransaction.CurrentIndex.ToUint64()

	return *transaction.currentIndex
}

// getter for the latestIndex (supports concurrency)
func (transaction *Transaction) GetLatestIndex() uint64 {
	transaction.latestIndexMutex.RLock()
	if transaction.latestIndex == nil {
		transaction.latestIndexMutex.RUnlock()
		transaction.latestIndexMutex.Lock()
		defer transaction.latestIndexMutex.Unlock()
		if transaction.latestIndex == nil {
			return transaction.parseLatestIndex()
		}
	}

	defer transaction.latestIndexMutex.RUnlock()

	return *transaction.latestIndex
}

// setter for the latestIndex (supports concurrency)
func (transaction *Transaction) SetLatestIndex(latestIndex uint64) {
	transaction.latestIndexMutex.RLock()
	if transaction.latestIndex == nil || *transaction.latestIndex != latestIndex {
		transaction.latestIndexMutex.RUnlock()
		transaction.latestIndexMutex.Lock()
		defer transaction.latestIndexMutex.Unlock()
		if transaction.latestIndex == nil || *transaction.latestIndex != latestIndex {
			*transaction.latestIndex = latestIndex

			transaction.SetModified(true)
		}
	} else {
		transaction.latestIndexMutex.RUnlock()
	}
}

// restores the latestIndex from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseLatestIndex() uint64 {
	transaction.latestIndexMutex.Lock()
	defer transaction.latestIndexMutex.Unlock()

	return transaction.parseLatestIndex()
}

// parses the latestIndex from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseLatestIndex() uint64 {
	*transaction.latestIndex = transaction.rawTransaction.LatestIndex.ToUint64()

	return *transaction.latestIndex
}

// getter for the bundleHash (supports concurrency)
func (transaction *Transaction) GetBundleHash() ternary.Trinary {
	transaction.bundleHashMutex.RLock()
	if transaction.bundleHash == nil {
		transaction.bundleHashMutex.RUnlock()
		transaction.bundleHashMutex.Lock()
		defer transaction.bundleHashMutex.Unlock()
		if transaction.bundleHash == nil {
			return transaction.parseBundleHash()
		}
	}

	defer transaction.bundleHashMutex.RUnlock()

	return *transaction.bundleHash
}

// setter for the bundleHash (supports concurrency)
func (transaction *Transaction) SetBundleHash(bundleHash ternary.Trinary) {
	transaction.bundleHashMutex.RLock()
	if transaction.bundleHash == nil || *transaction.bundleHash != bundleHash {
		transaction.bundleHashMutex.RUnlock()
		transaction.bundleHashMutex.Lock()
		defer transaction.bundleHashMutex.Unlock()
		if transaction.bundleHash == nil || *transaction.bundleHash != bundleHash {
			*transaction.bundleHash = bundleHash

			transaction.SetModified(true)
		}
	} else {
		transaction.bundleHashMutex.RUnlock()
	}
}

// restores the bundleHash from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseBundleHash() ternary.Trinary {
	transaction.bundleHashMutex.Lock()
	defer transaction.bundleHashMutex.Unlock()

	return transaction.parseBundleHash()
}

// parses the bundleHash from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseBundleHash() ternary.Trinary {
	*transaction.bundleHash = transaction.rawTransaction.BundleHash.ToTrinary()

	return *transaction.bundleHash
}

// getter for the trunkTransactionHash (supports concurrency)
func (transaction *Transaction) GetTrunkTransactionHash() ternary.Trinary {
	transaction.trunkTransactionHashMutex.RLock()
	if transaction.trunkTransactionHash == nil {
		transaction.trunkTransactionHashMutex.RUnlock()
		transaction.trunkTransactionHashMutex.Lock()
		defer transaction.trunkTransactionHashMutex.Unlock()
		if transaction.trunkTransactionHash == nil {
			return transaction.parseTrunkTransactionHash()
		}
	}

	defer transaction.trunkTransactionHashMutex.RUnlock()

	return *transaction.trunkTransactionHash
}

// setter for the trunkTransactionHash (supports concurrency)
func (transaction *Transaction) SetTrunkTransactionHash(trunkTransactionHash ternary.Trinary) {
	transaction.trunkTransactionHashMutex.RLock()
	if transaction.trunkTransactionHash == nil || *transaction.trunkTransactionHash != trunkTransactionHash {
		transaction.trunkTransactionHashMutex.RUnlock()
		transaction.trunkTransactionHashMutex.Lock()
		defer transaction.trunkTransactionHashMutex.Unlock()
		if transaction.trunkTransactionHash == nil || *transaction.trunkTransactionHash != trunkTransactionHash {
			*transaction.trunkTransactionHash = trunkTransactionHash

			transaction.SetModified(true)
		}
	} else {
		transaction.trunkTransactionHashMutex.RUnlock()
	}
}

// restores the trunkTransactionHash from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseTrunkTransactionHash() ternary.Trinary {
	transaction.trunkTransactionHashMutex.Lock()
	defer transaction.trunkTransactionHashMutex.Unlock()

	return transaction.parseTrunkTransactionHash()
}

// parses the trunkTransactionHash from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseTrunkTransactionHash() ternary.Trinary {
	*transaction.trunkTransactionHash = transaction.rawTransaction.TrunkTransactionHash.ToTrinary()

	return *transaction.trunkTransactionHash
}

// getter for the branchTransactionHash (supports concurrency)
func (transaction *Transaction) GetBranchTransactionHash() ternary.Trinary {
	transaction.branchTransactionHashMutex.RLock()
	if transaction.branchTransactionHash == nil {
		transaction.branchTransactionHashMutex.RUnlock()
		transaction.branchTransactionHashMutex.Lock()
		defer transaction.branchTransactionHashMutex.Unlock()
		if transaction.branchTransactionHash == nil {
			return transaction.parseBranchTransactionHash()
		}
	}

	defer transaction.branchTransactionHashMutex.RUnlock()

	return *transaction.branchTransactionHash
}

// setter for the branchTransactionHash (supports concurrency)
func (transaction *Transaction) SetBranchTransactionHash(branchTransactionHash ternary.Trinary) {
	transaction.branchTransactionHashMutex.RLock()
	if transaction.branchTransactionHash == nil || *transaction.branchTransactionHash != branchTransactionHash {
		transaction.branchTransactionHashMutex.RUnlock()
		transaction.branchTransactionHashMutex.Lock()
		defer transaction.branchTransactionHashMutex.Unlock()
		if transaction.branchTransactionHash == nil || *transaction.branchTransactionHash != branchTransactionHash {
			*transaction.branchTransactionHash = branchTransactionHash

			transaction.SetModified(true)
		}
	} else {
		transaction.branchTransactionHashMutex.RUnlock()
	}
}

// restores the branchTransactionHash from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseBranchTransactionHash() ternary.Trinary {
	transaction.branchTransactionHashMutex.Lock()
	defer transaction.branchTransactionHashMutex.Unlock()

	return transaction.parseBranchTransactionHash()
}

// parses the branchTransactionHash from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseBranchTransactionHash() ternary.Trinary {
	*transaction.branchTransactionHash = transaction.rawTransaction.BranchTransactionHash.ToTrinary()

	return *transaction.branchTransactionHash
}

// getter for the tag (supports concurrency)
func (transaction *Transaction) GetTag() ternary.Trinary {
	transaction.tagMutex.RLock()
	if transaction.tag == nil {
		transaction.tagMutex.RUnlock()
		transaction.tagMutex.Lock()
		defer transaction.tagMutex.Unlock()
		if transaction.tag == nil {
			return transaction.parseTag()
		}
	}

	defer transaction.tagMutex.RUnlock()

	return *transaction.tag
}

// setter for the tag (supports concurrency)
func (transaction *Transaction) SetTag(tag ternary.Trinary) {
	transaction.tagMutex.RLock()
	if transaction.tag == nil || *transaction.tag != tag {
		transaction.tagMutex.RUnlock()
		transaction.tagMutex.Lock()
		defer transaction.tagMutex.Unlock()
		if transaction.tag == nil || *transaction.tag != tag {
			*transaction.tag = tag

			transaction.SetModified(true)
		}
	} else {
		transaction.tagMutex.RUnlock()
	}
}

// restores the tag from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseTag() ternary.Trinary {
	transaction.tagMutex.Lock()
	defer transaction.tagMutex.Unlock()

	return transaction.parseTag()
}

// parses the tag from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseTag() ternary.Trinary {
	*transaction.tag = transaction.rawTransaction.Tag.ToTrinary()

	return *transaction.tag
}

// getter for the nonce (supports concurrency)
func (transaction *Transaction) GetNonce() ternary.Trinary {
	transaction.nonceMutex.RLock()
	if transaction.nonce == nil {
		transaction.nonceMutex.RUnlock()
		transaction.nonceMutex.Lock()
		defer transaction.nonceMutex.Unlock()
		if transaction.nonce == nil {
			return transaction.parseNonce()
		}
	}

	defer transaction.nonceMutex.RUnlock()

	return *transaction.nonce
}

// setter for the nonce (supports concurrency)
func (transaction *Transaction) SetNonce(nonce ternary.Trinary) {
	transaction.nonceMutex.RLock()
	if transaction.nonce == nil || *transaction.nonce != nonce {
		transaction.nonceMutex.RUnlock()
		transaction.nonceMutex.Lock()
		defer transaction.nonceMutex.Unlock()
		if transaction.nonce == nil || *transaction.nonce != nonce {
			*transaction.nonce = nonce

			transaction.SetModified(true)
		}
	} else {
		transaction.nonceMutex.RUnlock()
	}
}

// restores the nonce from the underlying raw transaction (supports concurrency)
func (transaction *Transaction) ParseNonce() ternary.Trinary {
	transaction.nonceMutex.Lock()
	defer transaction.nonceMutex.Unlock()

	return transaction.parseNonce()
}

// parses the nonce from the underlying raw transaction (without locking - internal usage)
func (transaction *Transaction) parseNonce() ternary.Trinary {
	*transaction.nonce = transaction.rawTransaction.Nonce.ToTrinary()

	return *transaction.nonce
}

// returns true if the transaction contains unsaved changes (supports concurrency)
func (transaction *Transaction) GetModified() bool {
	transaction.modifiedMutex.RLock()
	defer transaction.modifiedMutex.RUnlock()

	return transaction.modified
}

// sets the modified flag which controls if a transaction is going to be saved (supports concurrency)
func (transaction *Transaction) SetModified(modified bool) {
	transaction.modifiedMutex.Lock()
	defer transaction.modifiedMutex.Unlock()

	transaction.modified = modified
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region database functions ///////////////////////////////////////////////////////////////////////////////////////////

func (transaction *Transaction) Store() errors.IdentifiableError {
	if err := transactionDatabase.Set(transaction.rawTransaction.Hash.ToBytes(), transaction.rawTransaction.Bytes); err != nil {
		return ErrDatabaseError.Derive(err, "failed to store the transaction")
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
