package approvers

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/model"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

type Approvers struct {
	hash        trinary.Trytes
	hashes      map[trinary.Trytes]bool
	hashesMutex sync.RWMutex
	modified    bool
}

func New(hash trinary.Trytes) *Approvers {
	return &Approvers{
		hash:     hash,
		hashes:   make(map[trinary.Trytes]bool),
		modified: false,
	}
}

// region public methods with locking //////////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) Add(transactionHash trinary.Trytes) {
	approvers.hashesMutex.Lock()
	approvers.add(transactionHash)
	approvers.hashesMutex.Unlock()
}

func (approvers *Approvers) Remove(approverHash trinary.Trytes) {
	approvers.hashesMutex.Lock()
	approvers.remove(approverHash)
	approvers.hashesMutex.Unlock()
}

func (approvers *Approvers) GetHashes() (result []trinary.Trytes) {
	approvers.hashesMutex.RLock()
	result = approvers.getHashes()
	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) GetHash() (result trinary.Trytes) {
	approvers.hashesMutex.RLock()
	result = approvers.hash
	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) GetModified() bool {
	return true
}

func (approvers *Approvers) SetModified(modified bool) {
}

func (approvers *Approvers) Marshal() (result []byte) {
	approvers.hashesMutex.RLock()

	result = make([]byte, MARSHALED_APPROVERS_MIN_SIZE+len(approvers.hashes)*MARSHALED_APPROVERS_HASH_SIZE)

	binary.BigEndian.PutUint64(result[MARSHALED_APPROVERS_HASHES_COUNT_START:MARSHALED_APPROVERS_HASHES_COUNT_END], uint64(len(approvers.hashes)))

	copy(result[MARSHALED_APPROVERS_HASH_START:MARSHALED_APPROVERS_HASH_END], typeutils.StringToBytes(approvers.hash))

	i := 0
	for hash := range approvers.hashes {
		var HASH_START = MARSHALED_APPROVERS_HASHES_START + i*(MARSHALED_APPROVERS_HASH_SIZE)
		var HASH_END = HASH_START + MARSHALED_APPROVERS_HASH_SIZE

		copy(result[HASH_START:HASH_END], typeutils.StringToBytes(hash))

		i++
	}

	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) Unmarshal(data []byte) error {
	dataLen := len(data)

	if dataLen < MARSHALED_APPROVERS_MIN_SIZE {
		return fmt.Errorf("%w: marshaled approvers are too short", model.ErrMarshalFailed)
	}

	hashesCount := binary.BigEndian.Uint64(data[MARSHALED_APPROVERS_HASHES_COUNT_START:MARSHALED_APPROVERS_HASHES_COUNT_END])

	if dataLen < MARSHALED_APPROVERS_MIN_SIZE+int(hashesCount)*MARSHALED_APPROVERS_HASH_SIZE {
		return fmt.Errorf("%w: marshaled approvers are too short for %d approvers", model.ErrMarshalFailed, hashesCount)
	}

	approvers.hashesMutex.Lock()

	approvers.hash = trinary.Trytes(typeutils.BytesToString(data[MARSHALED_APPROVERS_HASH_START:MARSHALED_APPROVERS_HASH_END]))
	approvers.hashes = make(map[trinary.Trytes]bool, hashesCount)
	for i := uint64(0); i < hashesCount; i++ {
		var HASH_START = MARSHALED_APPROVERS_HASHES_START + i*(MARSHALED_APPROVERS_HASH_SIZE)
		var HASH_END = HASH_START + MARSHALED_APPROVERS_HASH_SIZE

		approvers.hashes[trinary.Trytes(typeutils.BytesToString(data[HASH_START:HASH_END]))] = true
	}

	approvers.hashesMutex.Unlock()

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region private methods without locking //////////////////////////////////////////////////////////////////////////////

func (approvers *Approvers) add(transactionHash trinary.Trytes) {
	if _, exists := approvers.hashes[transactionHash]; !exists {
		approvers.hashes[transactionHash] = true
		approvers.modified = true
	}
}

func (approvers *Approvers) remove(approverHash trinary.Trytes) {
	if _, exists := approvers.hashes[approverHash]; exists {
		delete(approvers.hashes, approverHash)
		approvers.modified = true
	}
}

func (approvers *Approvers) getHashes() (result []trinary.Trytes) {
	result = make([]trinary.Trytes, len(approvers.hashes))

	counter := 0
	for hash := range approvers.hashes {
		result[counter] = hash

		counter++
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
