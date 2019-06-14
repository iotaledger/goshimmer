package tangle

import (
	"encoding/binary"
	"strconv"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/typeconversion"
)

// region global public api ////////////////////////////////////////////////////////////////////////////////////////////

var approversCache = datastructure.NewLRUCache(METADATA_CACHE_SIZE)

func StoreApprovers(approvers *Approvers) {
	hash := approvers.GetHash()

	approversCache.Set(hash, approvers)
}

func GetApprovers(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *Approvers) (result *Approvers, err errors.IdentifiableError) {
	if approvers := approversCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if result, err = getApproversFromDatabase(transactionHash); err == nil && result == nil && len(computeIfAbsent) >= 1 {
			result = computeIfAbsent[0](transactionHash)
		}

		return result
	}); approvers != nil && approvers.(*Approvers) != nil {
		result = approvers.(*Approvers)
	}

	return
}

func ContainsApprovers(transactionHash ternary.Trinary) (result bool, err errors.IdentifiableError) {
	if approversCache.Contains(transactionHash) {
		result = true
	} else {
		result, err = databaseContainsApprovers(transactionHash)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type Approvers struct {
	hash        ternary.Trinary
	hashes      map[ternary.Trinary]bool
	hashesMutex sync.RWMutex
	modified    bool
}

func NewApprovers(hash ternary.Trinary) *Approvers {
	return &Approvers{
		hash:     hash,
		hashes:   make(map[ternary.Trinary]bool),
		modified: false,
	}
}

// region public methods with locking //////////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) Add(transactionHash ternary.Trinary) {
	approvers.hashesMutex.Lock()
	approvers.add(transactionHash)
	approvers.hashesMutex.Unlock()
}

func (approvers *Approvers) Remove(approverHash ternary.Trinary) {
	approvers.hashesMutex.Lock()
	approvers.remove(approverHash)
	approvers.hashesMutex.Unlock()
}

func (approvers *Approvers) GetHashes() (result []ternary.Trinary) {
	approvers.hashesMutex.RLock()
	result = approvers.getHashes()
	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) GetHash() (result ternary.Trinary) {
	approvers.hashesMutex.RLock()
	result = approvers.hash
	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) Marshal() (result []byte) {
	result = make([]byte, MARSHALLED_APPROVERS_MIN_SIZE+len(approvers.hashes)*MARSHALLED_APPROVERS_HASH_SIZE)

	approvers.hashesMutex.RLock()

	binary.BigEndian.PutUint64(result[MARSHALLED_APPROVERS_HASHES_COUNT_START:MARSHALLED_APPROVERS_HASHES_COUNT_END], uint64(len(approvers.hashes)))

	copy(result[MARSHALLED_APPROVERS_HASH_START:MARSHALLED_APPROVERS_HASH_END], approvers.hash.CastToBytes())

	i := 0
	for hash := range approvers.hashes {
		var HASH_START = MARSHALLED_APPROVERS_HASHES_START + i*(MARSHALLED_APPROVERS_HASH_SIZE)
		var HASH_END = HASH_START * MARSHALLED_APPROVERS_HASH_SIZE

		copy(result[HASH_START:HASH_END], hash.CastToBytes())

		i++
	}

	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) Unmarshal(data []byte) (err errors.IdentifiableError) {
	dataLen := len(data)

	if dataLen <= MARSHALLED_APPROVERS_MIN_SIZE {
		return ErrMarshallFailed.Derive(errors.New("unmarshall failed"), "marshalled approvers are too short")
	}

	hashesCount := binary.BigEndian.Uint64(data[MARSHALLED_APPROVERS_HASHES_COUNT_START:MARSHALLED_APPROVERS_HASHES_COUNT_END])

	if dataLen <= MARSHALLED_APPROVERS_MIN_SIZE+int(hashesCount)*MARSHALLED_APPROVERS_HASH_SIZE {
		return ErrMarshallFailed.Derive(errors.New("unmarshall failed"), "marshalled approvers are too short for "+strconv.FormatUint(hashesCount, 10)+" approvers")
	}

	approvers.hashesMutex.Lock()

	approvers.hash = ternary.Trinary(typeconversion.BytesToString(data[MARSHALLED_APPROVERS_HASH_START:MARSHALLED_APPROVERS_HASH_END]))
	approvers.hashes = make(map[ternary.Trinary]bool, hashesCount)
	for i := uint64(0); i < hashesCount; i++ {
		var HASH_START = MARSHALLED_APPROVERS_HASHES_START + i*(MARSHALLED_APPROVERS_HASH_SIZE)
		var HASH_END = HASH_START * MARSHALLED_APPROVERS_HASH_SIZE

		approvers.hashes[ternary.Trinary(typeconversion.BytesToString(data[HASH_START:HASH_END]))] = true
	}

	approvers.hashesMutex.Unlock()

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	MARSHALLED_APPROVERS_HASHES_COUNT_START = 0
	MARSHALLED_APPROVERS_HASH_START         = MARSHALLED_APPROVERS_HASHES_COUNT_END
	MARSHALLED_APPROVERS_HASHES_START       = MARSHALLED_APPROVERS_HASH_END

	MARSHALLED_APPROVERS_HASHES_COUNT_END = MARSHALLED_APPROVERS_HASHES_COUNT_START + MARSHALLED_APPROVERS_HASHES_COUNT_SIZE
	MARSHALLED_APPROVERS_HASH_END         = MARSHALLED_APPROVERS_HASH_START + MARSHALLED_APPROVERS_HASH_SIZE

	MARSHALLED_APPROVERS_HASHES_COUNT_SIZE = 8
	MARSHALLED_APPROVERS_HASH_SIZE         = 81
	MARSHALLED_APPROVERS_MIN_SIZE          = MARSHALLED_APPROVERS_HASHES_COUNT_SIZE + MARSHALLED_APPROVERS_HASH_SIZE
)

// region private methods without locking //////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) add(transactionHash ternary.Trinary) {
	if _, exists := approvers.hashes[transactionHash]; !exists {
		approvers.hashes[transactionHash] = true
		approvers.modified = true
	}
}

func (approvers *Approvers) remove(approverHash ternary.Trinary) {
	if _, exists := approvers.hashes[approverHash]; exists {
		delete(approvers.hashes, approverHash)
		approvers.modified = true
	}
}

func (approvers *Approvers) getHashes() (result []ternary.Trinary) {
	result = make([]ternary.Trinary, len(approvers.hashes))

	counter := 0
	for hash := range approvers.hashes {
		result[counter] = hash

		counter++
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) Store(approverHash ternary.Trinary) {
	approvers.hashesMutex.Lock()
	approvers.modified = false
	approvers.hashesMutex.Unlock()
}

func getApproversFromDatabase(transactionHash ternary.Trinary) (result *Approvers, err errors.IdentifiableError) {
	approversData, dbErr := approversDatabase.Get(transactionHash.CastToBytes())
	if dbErr != nil {
		if dbErr != badger.ErrKeyNotFound {
			err = ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}

		return
	}

	result = NewApprovers(transactionHash)
	if err = result.Unmarshal(approversData); err != nil {
		result = nil
	}

	return
}

func databaseContainsApprovers(transactionHash ternary.Trinary) (bool, errors.IdentifiableError) {
	if result, err := approversDatabase.Contains(transactionHash.CastToBytes()); err != nil {
		return false, ErrDatabaseError.Derive(err, "failed to check if the transaction exists")
	} else {
		return result, nil
	}
}
