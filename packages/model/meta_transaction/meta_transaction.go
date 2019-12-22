package meta_transaction

import (
	"fmt"
	"sync"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/curl"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"
)

type MetaTransaction struct {
	hash            *trinary.Trytes
	weightMagnitude int

	shardMarker           *trinary.Trytes
	trunkTransactionHash  *trinary.Trytes
	branchTransactionHash *trinary.Trytes
	head                  *bool
	tail                  *bool
	transactionType       *trinary.Trytes
	data                  trinary.Trits
	modified              bool
	nonce                 *trinary.Trytes

	hasherMutex                sync.RWMutex
	hashMutex                  sync.RWMutex
	shardMarkerMutex           sync.RWMutex
	trunkTransactionHashMutex  sync.RWMutex
	branchTransactionHashMutex sync.RWMutex
	headMutex                  sync.RWMutex
	tailMutex                  sync.RWMutex
	transactionTypeMutex       sync.RWMutex
	dataMutex                  sync.RWMutex
	bytesMutex                 sync.RWMutex
	modifiedMutex              sync.RWMutex
	nonceMutex                 sync.RWMutex

	trits trinary.Trits
	bytes []byte
}

func New() *MetaTransaction {
	return FromTrits(make(trinary.Trits, MARSHALED_TOTAL_SIZE))
}

func FromTrits(trits trinary.Trits) *MetaTransaction {
	return &MetaTransaction{
		trits: trits,
	}
}

func FromBytes(bytes []byte) (result *MetaTransaction) {
	trits, err := trinary.BytesToTrits(bytes)
	if err != nil {
		panic(err)
	}

	result = FromTrits(trits[:MARSHALED_TOTAL_SIZE])
	result.bytes = bytes

	return
}

func (this *MetaTransaction) BlockHasher() {
	this.hasherMutex.RLock()
}

func (this *MetaTransaction) UnblockHasher() {
	this.hasherMutex.RUnlock()
}

func (this *MetaTransaction) ReHash() {
	this.hashMutex.Lock()
	defer this.hashMutex.Unlock()
	this.hash = nil

	this.bytesMutex.Lock()
	defer this.bytesMutex.Unlock()
	this.bytes = nil
}

// retrieves the hash of the transaction
func (this *MetaTransaction) GetHash() (result trinary.Trytes) {
	this.hashMutex.RLock()
	if this.hash == nil {
		this.hashMutex.RUnlock()
		this.hashMutex.Lock()
		defer this.hashMutex.Unlock()
		if this.hash == nil {
			this.hasherMutex.Lock()
			this.computeHashDetails()
			this.hasherMutex.Unlock()
		}
	} else {
		defer this.hashMutex.RUnlock()
	}

	result = *this.hash

	return
}

// retrieves weight magnitude of the transaction (amount of pow invested)
func (this *MetaTransaction) GetWeightMagnitude() (result int) {
	this.hashMutex.RLock()
	if this.hash == nil {
		this.hashMutex.RUnlock()
		this.hashMutex.Lock()
		defer this.hashMutex.Unlock()
		if this.hash == nil {
			this.hasherMutex.Lock()
			this.computeHashDetails()
			this.hasherMutex.Unlock()
		}
	} else {
		defer this.hashMutex.RUnlock()
	}

	result = this.weightMagnitude

	return
}

// returns the trytes that are relevant for the transaction hash
func (this *MetaTransaction) getHashEssence() trinary.Trits {
	txTrits := this.trits

	// very dirty hack, to get an iota.go compatible size
	if len(txTrits) > consts.TransactionTrinarySize {
		panic("transaction too large")
	}
	essenceTrits := make([]int8, consts.TransactionTrinarySize)
	copy(essenceTrits[consts.TransactionTrinarySize-len(txTrits):], txTrits)

	return essenceTrits
}

// hashes the transaction using curl (without locking - internal usage)
func (this *MetaTransaction) computeHashDetails() {
	hashTrits, err := curl.HashTrits(this.getHashEssence())
	if err != nil {
		panic(err)
	}
	hashTrytes := trinary.MustTritsToTrytes(hashTrits)

	this.hash = &hashTrytes
	this.weightMagnitude = int(trinary.TrailingZeros(hashTrits))
}

// getter for the shard marker (supports concurrency)
func (this *MetaTransaction) GetShardMarker() (result trinary.Trytes) {
	this.shardMarkerMutex.RLock()
	if this.shardMarker == nil {
		this.shardMarkerMutex.RUnlock()
		this.shardMarkerMutex.Lock()
		defer this.shardMarkerMutex.Unlock()
		if this.shardMarker == nil {
			shardMarker := trinary.MustTritsToTrytes(this.trits[SHARD_MARKER_OFFSET:SHARD_MARKER_END])

			this.shardMarker = &shardMarker
		}
	} else {
		defer this.shardMarkerMutex.RUnlock()
	}

	result = *this.shardMarker

	return
}

// setter for the shard marker (supports concurrency)
func (this *MetaTransaction) SetShardMarker(shardMarker trinary.Trytes) bool {
	this.shardMarkerMutex.RLock()
	if this.shardMarker == nil || *this.shardMarker != shardMarker {
		this.shardMarkerMutex.RUnlock()
		this.shardMarkerMutex.Lock()
		defer this.shardMarkerMutex.Unlock()
		if this.shardMarker == nil || *this.shardMarker != shardMarker {
			this.shardMarker = &shardMarker

			this.hasherMutex.RLock()
			copy(this.trits[SHARD_MARKER_OFFSET:SHARD_MARKER_END], trinary.MustTrytesToTrits(shardMarker)[:SHARD_MARKER_SIZE])
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.shardMarkerMutex.RUnlock()
	}

	return false
}

// getter for the bundleHash (supports concurrency)
func (this *MetaTransaction) GetTrunkTransactionHash() (result trinary.Trytes) {
	this.trunkTransactionHashMutex.RLock()
	if this.trunkTransactionHash == nil {
		this.trunkTransactionHashMutex.RUnlock()
		this.trunkTransactionHashMutex.Lock()
		defer this.trunkTransactionHashMutex.Unlock()
		if this.trunkTransactionHash == nil {
			trunkTransactionHash := trinary.MustTritsToTrytes(this.trits[TRUNK_TRANSACTION_HASH_OFFSET:TRUNK_TRANSACTION_HASH_END])

			this.trunkTransactionHash = &trunkTransactionHash
		}
	} else {
		defer this.trunkTransactionHashMutex.RUnlock()
	}

	result = *this.trunkTransactionHash

	return
}

// setter for the trunkTransactionHash (supports concurrency)
func (this *MetaTransaction) SetTrunkTransactionHash(trunkTransactionHash trinary.Trytes) bool {
	this.trunkTransactionHashMutex.RLock()
	if this.trunkTransactionHash == nil || *this.trunkTransactionHash != trunkTransactionHash {
		this.trunkTransactionHashMutex.RUnlock()
		this.trunkTransactionHashMutex.Lock()
		defer this.trunkTransactionHashMutex.Unlock()
		if this.trunkTransactionHash == nil || *this.trunkTransactionHash != trunkTransactionHash {
			this.trunkTransactionHash = &trunkTransactionHash

			this.hasherMutex.RLock()
			copy(this.trits[TRUNK_TRANSACTION_HASH_OFFSET:TRUNK_TRANSACTION_HASH_END], trinary.MustTrytesToTrits(trunkTransactionHash)[:TRUNK_TRANSACTION_HASH_SIZE])
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.trunkTransactionHashMutex.RUnlock()
	}

	return false
}

// getter for the bundleHash (supports concurrency)
func (this *MetaTransaction) GetBranchTransactionHash() (result trinary.Trytes) {
	this.branchTransactionHashMutex.RLock()
	if this.branchTransactionHash == nil {
		this.branchTransactionHashMutex.RUnlock()
		this.branchTransactionHashMutex.Lock()
		defer this.branchTransactionHashMutex.Unlock()
		if this.branchTransactionHash == nil {
			branchTransactionHash := trinary.MustTritsToTrytes(this.trits[BRANCH_TRANSACTION_HASH_OFFSET:BRANCH_TRANSACTION_HASH_END])

			this.branchTransactionHash = &branchTransactionHash
		}
	} else {
		defer this.branchTransactionHashMutex.RUnlock()
	}

	result = *this.branchTransactionHash

	return
}

// setter for the trunkTransactionHash (supports concurrency)
func (this *MetaTransaction) SetBranchTransactionHash(branchTransactionHash trinary.Trytes) bool {
	this.branchTransactionHashMutex.RLock()
	if this.branchTransactionHash == nil || *this.branchTransactionHash != branchTransactionHash {
		this.branchTransactionHashMutex.RUnlock()
		this.branchTransactionHashMutex.Lock()
		defer this.branchTransactionHashMutex.Unlock()
		if this.branchTransactionHash == nil || *this.branchTransactionHash != branchTransactionHash {
			this.branchTransactionHash = &branchTransactionHash

			this.hasherMutex.RLock()
			copy(this.trits[BRANCH_TRANSACTION_HASH_OFFSET:BRANCH_TRANSACTION_HASH_END], trinary.MustTrytesToTrits(branchTransactionHash)[:BRANCH_TRANSACTION_HASH_SIZE])
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.branchTransactionHashMutex.RUnlock()
	}

	return false
}

// getter for the head flag (supports concurrency)
func (this *MetaTransaction) IsHead() (result bool) {
	this.headMutex.RLock()
	if this.head == nil {
		this.headMutex.RUnlock()
		this.headMutex.Lock()
		defer this.headMutex.Unlock()
		if this.head == nil {
			head := this.trits[HEAD_OFFSET] == 1

			this.head = &head
		}
	} else {
		defer this.headMutex.RUnlock()
	}

	result = *this.head

	return
}

// setter for the head flag (supports concurrency)
func (this *MetaTransaction) SetHead(head bool) bool {
	this.headMutex.RLock()
	if this.head == nil || *this.head != head {
		this.headMutex.RUnlock()
		this.headMutex.Lock()
		defer this.headMutex.Unlock()
		if this.head == nil || *this.head != head {
			this.head = &head

			this.hasherMutex.RLock()
			if head {
				this.trits[HEAD_OFFSET] = 1
			} else {
				this.trits[HEAD_OFFSET] = 0
			}
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.headMutex.RUnlock()
	}

	return false
}

// getter for the tail flag (supports concurrency)
func (this *MetaTransaction) IsTail() (result bool) {
	this.tailMutex.RLock()
	if this.tail == nil {
		this.tailMutex.RUnlock()
		this.tailMutex.Lock()
		defer this.tailMutex.Unlock()
		if this.tail == nil {
			tail := this.trits[TAIL_OFFSET] == 1

			this.tail = &tail
		}
	} else {
		defer this.tailMutex.RUnlock()
	}

	result = *this.tail

	return
}

// setter for the tail flag (supports concurrency)
func (this *MetaTransaction) SetTail(tail bool) bool {
	this.tailMutex.RLock()
	if this.tail == nil || *this.tail != tail {
		this.tailMutex.RUnlock()
		this.tailMutex.Lock()
		defer this.tailMutex.Unlock()
		if this.tail == nil || *this.tail != tail {
			this.tail = &tail

			this.hasherMutex.RLock()
			if tail {
				this.trits[TAIL_OFFSET] = 1
			} else {
				this.trits[TAIL_OFFSET] = 0
			}
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.tailMutex.RUnlock()
	}

	return false
}

// getter for the transaction type (supports concurrency)
func (this *MetaTransaction) GetTransactionType() (result trinary.Trytes) {
	this.transactionTypeMutex.RLock()
	if this.transactionType == nil {
		this.transactionTypeMutex.RUnlock()
		this.transactionTypeMutex.Lock()
		defer this.transactionTypeMutex.Unlock()
		if this.transactionType == nil {
			transactionType := trinary.MustTritsToTrytes(this.trits[TRANSACTION_TYPE_OFFSET:TRANSACTION_TYPE_END])

			this.transactionType = &transactionType
		}
	} else {
		defer this.transactionTypeMutex.RUnlock()
	}

	result = *this.transactionType

	return
}

// setter for the transaction type (supports concurrency)
func (this *MetaTransaction) SetTransactionType(transactionType trinary.Trytes) bool {
	this.transactionTypeMutex.RLock()
	if this.transactionType == nil || *this.transactionType != transactionType {
		this.transactionTypeMutex.RUnlock()
		this.transactionTypeMutex.Lock()
		defer this.transactionTypeMutex.Unlock()
		if this.transactionType == nil || *this.transactionType != transactionType {
			this.transactionType = &transactionType

			this.hasherMutex.RLock()
			copy(this.trits[TRANSACTION_TYPE_OFFSET:TRANSACTION_TYPE_END], trinary.MustTrytesToTrits(transactionType)[:TRANSACTION_TYPE_SIZE])
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.transactionTypeMutex.RUnlock()
	}

	return false
}

// getter for the data slice (supports concurrency)
func (this *MetaTransaction) GetData() (result trinary.Trits) {
	this.dataMutex.RLock()
	if this.data == nil {
		this.dataMutex.RUnlock()
		this.dataMutex.Lock()
		defer this.dataMutex.Unlock()
		if this.data == nil {
			this.data = this.trits[DATA_OFFSET:DATA_END]
		}
	} else {
		defer this.dataMutex.RUnlock()
	}

	result = this.data

	return
}

func (this *MetaTransaction) GetTrits() (result trinary.Trits) {
	result = make(trinary.Trits, len(this.trits))

	this.hasherMutex.Lock()
	copy(result, this.trits)
	this.hasherMutex.Unlock()

	return
}

func (this *MetaTransaction) GetBytes() (result []byte) {
	this.bytesMutex.RLock()
	if this.bytes == nil {
		this.bytesMutex.RUnlock()
		this.bytesMutex.Lock()
		defer this.bytesMutex.Unlock()

		this.hasherMutex.Lock()
		this.bytes = trinary.TritsToBytes(this.trits)
		this.hasherMutex.Unlock()
	} else {
		this.bytesMutex.RUnlock()
	}

	result = make([]byte, len(this.bytes))
	copy(result, this.bytes)

	return
}

func (this *MetaTransaction) GetNonce() trinary.Trytes {
	this.nonceMutex.RLock()
	if this.nonce == nil {
		this.nonceMutex.RUnlock()
		this.nonceMutex.Lock()
		defer this.nonceMutex.Unlock()
		if this.nonce == nil {
			nonce := trinary.MustTritsToTrytes(this.trits[NONCE_OFFSET:NONCE_END])

			this.nonce = &nonce
		}
	} else {
		defer this.nonceMutex.RUnlock()
	}

	return *this.nonce
}

func (this *MetaTransaction) SetNonce(nonce trinary.Trytes) bool {
	this.nonceMutex.RLock()
	if this.nonce == nil || *this.nonce != nonce {
		this.nonceMutex.RUnlock()
		this.nonceMutex.Lock()
		defer this.nonceMutex.Unlock()
		if this.nonce == nil || *this.nonce != nonce {
			this.nonce = &nonce

			this.hasherMutex.RLock()
			copy(this.trits[NONCE_OFFSET:NONCE_END], trinary.MustTrytesToTrits(nonce)[:NONCE_SIZE])
			this.hasherMutex.RUnlock()

			this.SetModified(true)
			this.ReHash()

			return true
		}
	} else {
		this.nonceMutex.RUnlock()
	}

	return false
}

// returns true if the transaction contains unsaved changes (supports concurrency)
func (this *MetaTransaction) GetModified() bool {
	this.modifiedMutex.RLock()
	defer this.modifiedMutex.RUnlock()

	return this.modified
}

// sets the modified flag which controls if a transaction is going to be saved (supports concurrency)
func (this *MetaTransaction) SetModified(modified bool) {
	this.modifiedMutex.Lock()
	defer this.modifiedMutex.Unlock()

	this.modified = modified
}

func (this *MetaTransaction) DoProofOfWork(mwm int) error {
	this.hasherMutex.Lock()
	powTrytes := trinary.MustTritsToTrytes(this.getHashEssence())
	_, pow := pow.GetFastestProofOfWorkImpl()
	nonce, err := pow(powTrytes, mwm)
	this.hasherMutex.Unlock()

	if err != nil {
		return errors.Wrap(err, "PoW failed")
	}
	this.SetNonce(nonce)

	return nil
}

func (this *MetaTransaction) Validate() error {
	// check that the weight magnitude is valid
	weightMagnitude := this.GetWeightMagnitude()
	if weightMagnitude < MIN_WEIGHT_MAGNITUDE {
		return fmt.Errorf("insufficient weight magnitude: got=%d, want=%d", weightMagnitude, MIN_WEIGHT_MAGNITUDE)
	}

	return nil
}
