package tangle

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/mr-tron/base58"
)

// Tangle represents the FPC test tangle that consists out of FPCTest payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

// New is the constructor of a Tangle and creates a new Tangle object from the given details.
func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		payloadStorage:         osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),

		Events: *newEvents(),
	}

	return
}

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *payload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

// GetPayload retrieves a payload from the object storage.
func (tangle *Tangle) GetPayload(payloadID payload.ID) *payload.CachedPayload {
	return &payload.CachedPayload{CachedObject: tangle.payloadStorage.Load(payloadID.Bytes())}
}

// IDFromBase58 creates a new ID from a base58 encoded string.
func IDFromBase58(base58String string) (ID payload.ID, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != payload.IDLength {
		err = fmt.Errorf("base58 encoded string does not match the length of a FPCTest ID")

		return
	}

	// copy bytes to result
	copy(ID[:], bytes)

	return
}

// PayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) PayloadMetadata(payloadID payload.ID) *CachedPayloadMetadata {
	return &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Load(payloadID.Bytes())}
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.storePayloadWorkerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	tangle.payloadStorage.Shutdown()
	tangle.payloadMetadataStorage.Shutdown()

	return tangle
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (tangle *Tangle) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
	} {
		if err := storage.Prune(); err != nil {
			return err
		}
	}

	return nil
}

// storePayloadWorker is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) storePayloadWorker(payloadToStore *payload.Payload) {
	// store the payload and transaction models
	cachedPayload, cachedPayloadMetadata, payloadStored := tangle.storePayload(payloadToStore)
	if !payloadStored {
		// abort if we have seen the payload already
		return
	}

	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedPayloadMetadata)

}

func (tangle *Tangle) storePayload(payloadToStore *payload.Payload) (cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, payloadStored bool) {
	storedTransaction, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payloadToStore)
	if !transactionIsNew {
		return
	}

	cachedPayload = &payload.CachedPayload{CachedObject: storedTransaction}
	cachedMetadata = &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Store(NewPayloadMetadata(payloadToStore.ID()))}
	payloadStored = true

	return
}

// isPayloadSolid returns true if the given payload is solid. A payload is considered to be solid solid, if it is either
// already marked as solid or if its referenced payloads are marked as solid.
func (tangle *Tangle) isPayloadLiked(payload *payload.Payload, metadata *PayloadMetadata) bool {
	if payload == nil || payload.IsDeleted() {
		return false
	}

	if metadata == nil || metadata.IsDeleted() {
		return false
	}

	return metadata.IsLiked()
}
