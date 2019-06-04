package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/bitutils"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/typeconversion"
)

// region type definition and constructor //////////////////////////////////////////////////////////////////////////////

type TransactionMetadata struct {
	hash              ternary.Trinary
	hashMutex         sync.RWMutex
	receivedTime      time.Time
	receivedTimeMutex sync.RWMutex
	solid             bool
	solidMutex        sync.RWMutex
	liked             bool
	likedMutex        sync.RWMutex
	finalized         bool
	finalizedMutex    sync.RWMutex
	modified          bool
	modifiedMutex     sync.RWMutex
}

func NewTransactionMetadata(hash ternary.Trinary) *TransactionMetadata {
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

func (metaData *TransactionMetadata) GetHash() ternary.Trinary {
	metaData.hashMutex.RLock()
	defer metaData.hashMutex.RUnlock()

	return metaData.hash
}

func (metaData *TransactionMetadata) SetHash(hash ternary.Trinary) {
	metaData.hashMutex.RLock()
	if metaData.hash != hash {
		metaData.hashMutex.RUnlock()
		metaData.hashMutex.Lock()
		defer metaData.hashMutex.Unlock()
		if metaData.hash != hash {
			metaData.hash = hash

			metaData.SetModified(true)
		}
	} else {
		metaData.hashMutex.RUnlock()
	}
}

func (metaData *TransactionMetadata) GetReceivedTime() time.Time {
	metaData.receivedTimeMutex.RLock()
	defer metaData.receivedTimeMutex.RUnlock()

	return metaData.receivedTime
}

func (metaData *TransactionMetadata) SetReceivedTime(receivedTime time.Time) {
	metaData.receivedTimeMutex.RLock()
	if metaData.receivedTime != receivedTime {
		metaData.receivedTimeMutex.RUnlock()
		metaData.receivedTimeMutex.Lock()
		defer metaData.receivedTimeMutex.Unlock()
		if metaData.receivedTime != receivedTime {
			metaData.receivedTime = receivedTime

			metaData.SetModified(true)
		}
	} else {
		metaData.receivedTimeMutex.RUnlock()
	}
}

func (metaData *TransactionMetadata) GetSolid() bool {
	metaData.solidMutex.RLock()
	defer metaData.solidMutex.RUnlock()

	return metaData.solid
}

func (metaData *TransactionMetadata) SetSolid(solid bool) {
	metaData.solidMutex.RLock()
	if metaData.solid != solid {
		metaData.solidMutex.RUnlock()
		metaData.solidMutex.Lock()
		defer metaData.solidMutex.Unlock()
		if metaData.solid != solid {
			metaData.solid = solid

			metaData.SetModified(true)
		}
	} else {
		metaData.solidMutex.RUnlock()
	}
}

func (metaData *TransactionMetadata) GetLiked() bool {
	metaData.likedMutex.RLock()
	defer metaData.likedMutex.RUnlock()

	return metaData.liked
}

func (metaData *TransactionMetadata) SetLiked(liked bool) {
	metaData.likedMutex.RLock()
	if metaData.liked != liked {
		metaData.likedMutex.RUnlock()
		metaData.likedMutex.Lock()
		defer metaData.likedMutex.Unlock()
		if metaData.liked != liked {
			metaData.liked = liked

			metaData.SetModified(true)
		}
	} else {
		metaData.likedMutex.RUnlock()
	}
}

func (metaData *TransactionMetadata) GetFinalized() bool {
	metaData.finalizedMutex.RLock()
	defer metaData.finalizedMutex.RUnlock()

	return metaData.finalized
}

func (metaData *TransactionMetadata) SetFinalized(finalized bool) {
	metaData.finalizedMutex.RLock()
	if metaData.finalized != finalized {
		metaData.finalizedMutex.RUnlock()
		metaData.finalizedMutex.Lock()
		defer metaData.finalizedMutex.Unlock()
		if metaData.finalized != finalized {
			metaData.finalized = finalized

			metaData.SetModified(true)
		}
	} else {
		metaData.finalizedMutex.RUnlock()
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

// region marshalling functions ////////////////////////////////////////////////////////////////////////////////////////

func (metadata *TransactionMetadata) Marshal() ([]byte, errors.IdentifiableError) {
	marshalledMetadata := make([]byte, MARSHALLED_TOTAL_SIZE)

	metadata.receivedTimeMutex.RLock()
	defer metadata.receivedTimeMutex.RUnlock()
	metadata.solidMutex.RLock()
	defer metadata.solidMutex.RUnlock()
	metadata.likedMutex.RLock()
	defer metadata.likedMutex.RUnlock()
	metadata.finalizedMutex.RLock()
	defer metadata.finalizedMutex.RUnlock()

	copy(marshalledMetadata[MARSHALLED_HASH_START:MARSHALLED_HASH_END], metadata.hash.CastToBytes())

	marshalledReceivedTime, err := metadata.receivedTime.MarshalBinary()
	if err != nil {
		return nil, ErrMarshallFailed.Derive(err, "failed to marshal received time")
	}
	copy(marshalledMetadata[MARSHALLED_RECEIVED_TIME_START:MARSHALLED_RECEIVED_TIME_END], marshalledReceivedTime)

	var booleanFlags bitutils.BitMask
	if metadata.solid {
		booleanFlags = booleanFlags.SetFlag(0)
	}
	if metadata.liked {
		booleanFlags = booleanFlags.SetFlag(1)
	}
	if metadata.finalized {
		booleanFlags = booleanFlags.SetFlag(2)
	}
	marshalledMetadata[MARSHALLED_FLAGS_START] = byte(booleanFlags)

	return marshalledMetadata, nil
}

func (metadata *TransactionMetadata) Unmarshal(data []byte) errors.IdentifiableError {
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

	metadata.hash = ternary.Trinary(typeconversion.BytesToString(data[MARSHALLED_HASH_START:MARSHALLED_HASH_END]))

	if err := metadata.receivedTime.UnmarshalBinary(data[MARSHALLED_RECEIVED_TIME_START:MARSHALLED_RECEIVED_TIME_END]); err != nil {
		return ErrUnmarshalFailed.Derive(err, "could not unmarshal the received time")
	}

	booleanFlags := bitutils.BitMask(data[MARSHALLED_FLAGS_START])
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

// region database functions ///////////////////////////////////////////////////////////////////////////////////////////

func (metadata *TransactionMetadata) Store() errors.IdentifiableError {
	if metadata.GetModified() {
		marshalledMetadata, err := metadata.Marshal()
		if err != nil {
			return err
		}

		if err := transactionMetadataDatabase.Set(metadata.GetHash().CastToBytes(), marshalledMetadata); err != nil {
			return ErrDatabaseError.Derive(err, "failed to store the transaction")
		}

		metadata.SetModified(false)
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

const (
	MARSHALLED_HASH_START          = 0
	MARSHALLED_RECEIVED_TIME_START = MARSHALLED_HASH_END
	MARSHALLED_FLAGS_START         = MARSHALLED_RECEIVED_TIME_END

	MARSHALLED_HASH_END          = MARSHALLED_HASH_START + MARSHALLED_HASH_SIZE
	MARSHALLED_RECEIVED_TIME_END = MARSHALLED_RECEIVED_TIME_START + MARSHALLED_RECEIVED_TIME_SIZE
	MARSHALLED_FLAGS_END         = MARSHALLED_FLAGS_START + MARSHALLED_FLAGS_SIZE

	MARSHALLED_HASH_SIZE          = 81
	MARSHALLED_RECEIVED_TIME_SIZE = 15
	MARSHALLED_FLAGS_SIZE         = 1

	MARSHALLED_TOTAL_SIZE = MARSHALLED_FLAGS_END
)

// endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////
