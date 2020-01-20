package transactionmetadata

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/model"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

// region type definition and constructor //////////////////////////////////////////////////////////////////////////////

type TransactionMetadata struct {
	hash                trinary.Trytes
	hashMutex           sync.RWMutex
	bundleHeadHash      trinary.Trytes
	bundleHeadHashMutex sync.RWMutex
	receivedTime        time.Time
	receivedTimeMutex   sync.RWMutex
	solid               bool
	solidMutex          sync.RWMutex
	liked               bool
	likedMutex          sync.RWMutex
	finalized           bool
	finalizedMutex      sync.RWMutex
	modified            bool
	modifiedMutex       sync.RWMutex
}

func New(hash trinary.Trytes) *TransactionMetadata {
	return &TransactionMetadata{
		hash:         hash,
		receivedTime: time.Now(),
		solid:        false,
		liked:        false,
		finalized:    false,
		modified:     true,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region getters and setters //////////////////////////////////////////////////////////////////////////////////////////

func (metadata *TransactionMetadata) GetHash() trinary.Trytes {
	metadata.hashMutex.RLock()
	defer metadata.hashMutex.RUnlock()

	return metadata.hash
}

func (metadata *TransactionMetadata) SetHash(hash trinary.Trytes) {
	metadata.hashMutex.RLock()
	if metadata.hash != hash {
		metadata.hashMutex.RUnlock()
		metadata.hashMutex.Lock()
		defer metadata.hashMutex.Unlock()
		if metadata.hash != hash {
			metadata.hash = hash

			metadata.SetModified(true)
		}
	} else {
		metadata.hashMutex.RUnlock()
	}
}

func (metadata *TransactionMetadata) GetBundleHeadHash() trinary.Trytes {
	metadata.bundleHeadHashMutex.RLock()
	defer metadata.bundleHeadHashMutex.RUnlock()

	return metadata.bundleHeadHash
}

func (metadata *TransactionMetadata) SetBundleHeadHash(bundleTailHash trinary.Trytes) {
	metadata.bundleHeadHashMutex.RLock()
	if metadata.bundleHeadHash != bundleTailHash {
		metadata.bundleHeadHashMutex.RUnlock()
		metadata.bundleHeadHashMutex.Lock()
		defer metadata.bundleHeadHashMutex.Unlock()
		if metadata.bundleHeadHash != bundleTailHash {
			metadata.bundleHeadHash = bundleTailHash

			metadata.SetModified(true)
		}
	} else {
		metadata.bundleHeadHashMutex.RUnlock()
	}
}

func (metadata *TransactionMetadata) GetReceivedTime() time.Time {
	metadata.receivedTimeMutex.RLock()
	defer metadata.receivedTimeMutex.RUnlock()

	return metadata.receivedTime
}

func (metadata *TransactionMetadata) SetReceivedTime(receivedTime time.Time) {
	metadata.receivedTimeMutex.RLock()
	if metadata.receivedTime != receivedTime {
		metadata.receivedTimeMutex.RUnlock()
		metadata.receivedTimeMutex.Lock()
		defer metadata.receivedTimeMutex.Unlock()
		if metadata.receivedTime != receivedTime {
			metadata.receivedTime = receivedTime

			metadata.SetModified(true)
		}
	} else {
		metadata.receivedTimeMutex.RUnlock()
	}
}

func (metadata *TransactionMetadata) GetSolid() bool {
	metadata.solidMutex.RLock()
	defer metadata.solidMutex.RUnlock()

	return metadata.solid
}

func (metadata *TransactionMetadata) SetSolid(solid bool) bool {
	metadata.solidMutex.RLock()
	if metadata.solid != solid {
		metadata.solidMutex.RUnlock()
		metadata.solidMutex.Lock()
		defer metadata.solidMutex.Unlock()
		if metadata.solid != solid {
			metadata.solid = solid

			metadata.SetModified(true)

			return true
		}
	} else {
		metadata.solidMutex.RUnlock()
	}

	return false
}

func (metadata *TransactionMetadata) GetLiked() bool {
	metadata.likedMutex.RLock()
	defer metadata.likedMutex.RUnlock()

	return metadata.liked
}

func (metadata *TransactionMetadata) SetLiked(liked bool) {
	metadata.likedMutex.RLock()
	if metadata.liked != liked {
		metadata.likedMutex.RUnlock()
		metadata.likedMutex.Lock()
		defer metadata.likedMutex.Unlock()
		if metadata.liked != liked {
			metadata.liked = liked

			metadata.SetModified(true)
		}
	} else {
		metadata.likedMutex.RUnlock()
	}
}

func (metadata *TransactionMetadata) GetFinalized() bool {
	metadata.finalizedMutex.RLock()
	defer metadata.finalizedMutex.RUnlock()

	return metadata.finalized
}

func (metadata *TransactionMetadata) SetFinalized(finalized bool) {
	metadata.finalizedMutex.RLock()
	if metadata.finalized != finalized {
		metadata.finalizedMutex.RUnlock()
		metadata.finalizedMutex.Lock()
		defer metadata.finalizedMutex.Unlock()
		if metadata.finalized != finalized {
			metadata.finalized = finalized

			metadata.SetModified(true)
		}
	} else {
		metadata.finalizedMutex.RUnlock()
	}
}

// returns true if the transaction contains unsaved changes (supports concurrency)
func (metadata *TransactionMetadata) GetModified() bool {
	metadata.modifiedMutex.RLock()
	defer metadata.modifiedMutex.RUnlock()

	return metadata.modified
}

// sets the modified flag which controls if a transaction is going to be saved (supports concurrency)
func (metadata *TransactionMetadata) SetModified(modified bool) {
	metadata.modifiedMutex.Lock()
	defer metadata.modifiedMutex.Unlock()

	metadata.modified = modified
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region marshaling functions /////////////////////////////////////////////////////////////////////////////////////////

func (metadata *TransactionMetadata) Marshal() ([]byte, error) {
	marshaledMetadata := make([]byte, MARSHALED_TOTAL_SIZE)

	metadata.receivedTimeMutex.RLock()
	defer metadata.receivedTimeMutex.RUnlock()
	metadata.solidMutex.RLock()
	defer metadata.solidMutex.RUnlock()
	metadata.likedMutex.RLock()
	defer metadata.likedMutex.RUnlock()
	metadata.finalizedMutex.RLock()
	defer metadata.finalizedMutex.RUnlock()

	copy(marshaledMetadata[MARSHALED_HASH_START:MARSHALED_HASH_END], typeutils.StringToBytes(metadata.hash))

	marshaledReceivedTime, err := metadata.receivedTime.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal received time: %s", model.ErrMarshalFailed, err.Error())
	}
	copy(marshaledMetadata[MARSHALED_RECEIVED_TIME_START:MARSHALED_RECEIVED_TIME_END], marshaledReceivedTime)

	var booleanFlags bitmask.BitMask
	if metadata.solid {
		booleanFlags = booleanFlags.SetFlag(0)
	}
	if metadata.liked {
		booleanFlags = booleanFlags.SetFlag(1)
	}
	if metadata.finalized {
		booleanFlags = booleanFlags.SetFlag(2)
	}
	marshaledMetadata[MARSHALED_FLAGS_START] = byte(booleanFlags)

	return marshaledMetadata, nil
}

func (metadata *TransactionMetadata) Unmarshal(data []byte) error {
	metadata.hashMutex.Lock()
	defer metadata.hashMutex.Unlock()
	metadata.receivedTimeMutex.Lock()
	defer metadata.receivedTimeMutex.Unlock()
	metadata.solidMutex.Lock()
	defer metadata.solidMutex.Unlock()
	metadata.likedMutex.Lock()
	defer metadata.likedMutex.Unlock()
	metadata.finalizedMutex.Lock()
	defer metadata.finalizedMutex.Unlock()

	metadata.hash = typeutils.BytesToString(data[MARSHALED_HASH_START:MARSHALED_HASH_END])

	if err := metadata.receivedTime.UnmarshalBinary(data[MARSHALED_RECEIVED_TIME_START:MARSHALED_RECEIVED_TIME_END]); err != nil {
		return fmt.Errorf("%w: could not unmarshal the received time: %s", model.ErrUnmarshalFailed, err.Error())
	}

	booleanFlags := bitmask.BitMask(data[MARSHALED_FLAGS_START])
	if booleanFlags.HasFlag(0) {
		metadata.solid = true
	}
	if booleanFlags.HasFlag(1) {
		metadata.liked = true
	}
	if booleanFlags.HasFlag(2) {
		metadata.finalized = true
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
