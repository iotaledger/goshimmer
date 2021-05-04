package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	// allowedFutureBooking defines the duration in which messages ahead of the TangleTime can be forwarded to the scheduler.
	allowedFutureBooking = 10 * time.Minute
	// bucketGranularity defines the granularity of the time based buckets in seconds.
	bucketGranularity = 60
	bucketInboxSize   = 30
)

var allowedFutureBookingSeconds = int64(allowedFutureBooking.Seconds())

// Orderer is a Tangle component that makes sure that no messages too far ahead of the TangleTime are booked.
// This is necessary to basically replay the tangle data structure as it was constructed during syncing to avoid
// distortions in perceptions of the approval weight.
type Orderer struct {
	Events *OrdererEvents

	tangle         *Tangle
	shutdownSignal chan struct{}
	shutdownWG     sync.WaitGroup
	shutdownOnce   sync.Once

	bucketInboxMin int64
	bucketInbox    chan int64

	maxBucketScheduled     int64
	maxBucketScheduledCond *sync.Cond

	bucketsMap      map[int64]uint64
	bucketsMapMutex sync.Mutex
}

// NewOrderer is the constructor for Orderer.
func NewOrderer(tangle *Tangle) (orderer *Orderer) {
	orderer = &Orderer{
		Events: &OrdererEvents{
			MessageOrdered: events.NewEvent(MessageIDCaller),
		},
		tangle:                 tangle,
		shutdownSignal:         make(chan struct{}),
		bucketInbox:            make(chan int64, bucketInboxSize),
		maxBucketScheduledCond: sync.NewCond(&sync.Mutex{}),
		bucketsMap:             make(map[int64]uint64),
	}
	orderer.run()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (o *Orderer) Setup() {
	o.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(o.Order))
	o.tangle.TimeManager.Events.TimeUpdated.Attach(events.NewClosure(o.onTangleTimeUpdated))
}

// Shutdown shuts down the Orderer and persists its state.
func (o *Orderer) Shutdown() {
	o.shutdownOnce.Do(func() {
		close(o.shutdownSignal)
	})

	o.shutdownWG.Wait()
}

// Order is the main function of the Orderer, making sure that no message too far ahead of TangleTime gets scheduled.
func (o *Orderer) Order(messageID MessageID) {
	var issuingTime time.Time
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		issuingTime = message.IssuingTime()
	})

	if o.tangle.TimeManager.IsGenesis() {
		o.Events.MessageOrdered.Trigger(messageID)
		return
	}

	bucket := bucketTime(issuingTime)
	//fmt.Println("Order", messageID, bucket)
	// TODO: maybe this check is not necessary with maxBucketScheduled
	if issuingTime.Before(o.tangle.TimeManager.Time().Add(allowedFutureBooking)) {
		// Messages can be scheduled directly after their bucket has been fully scheduled to make sure messages are scheduled
		// in order as they got solid (parents are scheduled before children).
		o.awaitBucketScheduled(bucket)

		o.Events.MessageOrdered.Trigger(messageID)
		return
	}

	// Store message in corresponding bucket and remember order from solidifier with count.
	o.bucketsMapMutex.Lock()
	count := o.bucketsMap[bucket]
	o.bucketsMap[bucket] = count + 1
	o.bucketsMapMutex.Unlock()
	o.tangle.Storage.StoreBucketMessageID(bucket, count, messageID)
	//fmt.Println("STORED", messageID, bucket, count)
}

func (o *Orderer) awaitBucketScheduled(bucket int64) {
	o.maxBucketScheduledCond.L.Lock()
	for bucket >= o.maxBucketScheduled {
		//fmt.Println("WAIT")
		o.maxBucketScheduledCond.Wait()
	}
	o.maxBucketScheduledCond.L.Unlock()
}

// run runs the background thread that listens to TangleTime updates (through a channel) and then schedules messages
// as the TangleTime advances forward.
func (o *Orderer) run() {
	o.shutdownWG.Add(1)
	go func() {
		defer o.shutdownWG.Done()

		for {
			select {
			case <-o.shutdownSignal:
				return
			case bucket := <-o.bucketInbox:
				o.scheduleUntil(bucket)
			}
		}
	}()
}

// scheduleUntil schedules messages in stored buckets from lowestStoredBucket until the given bucketTime + allowedFutureBookingSeconds.
func (o *Orderer) scheduleUntil(bucket int64) {
	// iterate over all buckets up until allowed bucket
	for currentBucket := bucket; currentBucket < bucket+allowedFutureBookingSeconds; currentBucket += bucketGranularity {
		o.bucketsMapMutex.Lock()
		maxCount := o.bucketsMap[currentBucket]
		delete(o.bucketsMap, currentBucket)
		o.bucketsMapMutex.Unlock()
		// iterate over all messages in bucket and forward to scheduler in order
		for count := uint64(0); count < maxCount; count++ {
			//fmt.Println("scheduleUntil", bucket, currentBucket, maxCount)
			//fmt.Println("maxCount", maxCount, count)
			o.tangle.Storage.BucketMessageID(currentBucket, count).Consume(func(bucketMessageID *BucketMessageID) {
				o.Events.MessageOrdered.Trigger(bucketMessageID.MessageID())
				bucketMessageID.Delete()
			})
		}
	}

	// Signal all threads waiting for buckets
	o.maxBucketScheduledCond.L.Lock()
	o.maxBucketScheduled = bucket + allowedFutureBookingSeconds
	//fmt.Println("o.maxBucketScheduled", o.maxBucketScheduled)
	o.maxBucketScheduledCond.L.Unlock()
	o.maxBucketScheduledCond.Broadcast()
}

// onTangleTimeUpdated listens to TangleTime updates and pushes buckets to the channel so that they can be scheduled.
func (o *Orderer) onTangleTimeUpdated(tangleTime time.Time) {
	bucket := bucketTime(tangleTime)
	if bucket > o.bucketInboxMin {
		o.bucketInboxMin = bucket
		o.bucketInbox <- bucket
	}
	//fmt.Println("onTangleTimeUpdated", bucket)
}

// bucketTime converts a time to a bucket with bucketGranularity.
func bucketTime(t time.Time) int64 {
	return t.Unix() / bucketGranularity * bucketGranularity
}

// region OrdererEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// OrdererEvents represents events happening in the Orderer.
type OrdererEvents struct {
	// MessageOrdered is triggered when a message is ordered and thus ready to be scheduled.
	MessageOrdered *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BucketMessageID //////////////////////////////////////////////////////////////////////////////////////////////

// BucketMessageIDPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
//var BucketMessageIDPartitionKeys = objectstorage.PartitionKey(marshalutil.Int64Size, MessageIDLength)

// BucketMessageID is a data structure that maps a time bucket to a MessageID.
type BucketMessageID struct {
	bucketTime int64
	count      uint64
	messageID  MessageID

	objectstorage.StorableObjectFlags
}

// NewBucketMessageID creates a new BucketMessageID.
func NewBucketMessageID(bucketTime int64, count uint64, messageID MessageID) (bucketMessageID *BucketMessageID) {
	bucketMessageID = &BucketMessageID{
		bucketTime: bucketTime,
		count:      count,
		messageID:  messageID,
	}

	return
}

// BucketMessageIDFromBytes unmarshals a BucketMessageID object from a sequence of bytes.
func BucketMessageIDFromBytes(bytes []byte) (bucketMessageID *BucketMessageID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if bucketMessageID, err = BucketMessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BucketMessageID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BucketMessageIDFromMarshalUtil unmarshals a BucketMessageID object using a MarshalUtil (for easier unmarshaling).
func BucketMessageIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (bucketMessageID *BucketMessageID, err error) {
	bucketMessageID = &BucketMessageID{}

	if bucketMessageID.bucketTime, err = marshalUtil.ReadInt64(); err != nil {
		err = xerrors.Errorf("failed to parse bucket time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if bucketMessageID.count, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if bucketMessageID.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// BucketMessageIDFromObjectStorage restores a BucketMessageID object from the object storage.
func BucketMessageIDFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = BucketMessageIDFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse BucketMessageID from bytes: %w", err)
		return
	}

	return
}

// BucketTime returns the time of the bucket.
func (b *BucketMessageID) BucketTime() (bucketTime int64) {
	return b.bucketTime
}

// Count returns the count of the bucket.
func (b *BucketMessageID) Count() (count uint64) {
	return b.count
}

// MessageID returns the MessageID that belongs to the bucket time.
func (b *BucketMessageID) MessageID() (messageID MessageID) {
	return b.messageID
}

// Bytes returns a marshaled version of the BucketMessageID.
func (b *BucketMessageID) Bytes() (marshaledSequenceSupporters []byte) {
	return byteutils.ConcatBytes(b.ObjectStorageKey(), b.ObjectStorageValue())
}

// String returns a human readable version of the BucketMessageID.
func (b *BucketMessageID) String() string {
	return stringify.Struct("BucketMessageID",
		stringify.StructField("bucketTime", b.BucketTime()),
		stringify.StructField("count", b.Count()),
		stringify.StructField("MessageID", b.MessageID()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *BucketMessageID) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BucketMessageID) ObjectStorageKey() []byte {
	marshalUtil := marshalutil.New(marshalutil.Int64Size + marshalutil.Uint64Size)

	return marshalUtil.
		WriteInt64(b.bucketTime).
		WriteUint64(b.count).
		Bytes()
}

// ObjectStorageValue marshals the BucketMessageID into a sequence of bytes that are used as the value part in the
// object storage.
func (b *BucketMessageID) ObjectStorageValue() (value []byte) {
	return b.MessageID().Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &BucketMessageID{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedBucketMessageID ////////////////////////////////////////////////////////////////////////////////////////

// CachedBucketMessageID is a wrapper for a stored cached object representing a BucketMessageID.
type CachedBucketMessageID struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the CachedBucketMessageID into the underlying BucketMessageID.
// If stored object cannot be cast into a BucketMessageID or has been deleted, it returns nil.
func (c *CachedBucketMessageID) Unwrap() *BucketMessageID {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*BucketMessageID)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume consumes the CachedBucketMessageID.
// It releases the object when the callback is done.
// It returns true if the callback was called.
func (c *CachedBucketMessageID) Consume(consumer func(bucketMessageID *BucketMessageID), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*BucketMessageID))
	}, forceRelease...)
}

// String returns a human readable version of the CachedBucketMessageID.
func (c *CachedBucketMessageID) String() string {
	return stringify.Struct("CachedBucketMessageID",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// CachedBucketMessageIDs represents a collection of CachedBucketMessageID.
type CachedBucketMessageIDs []*CachedBucketMessageID

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedBucketMessageIDs CachedBucketMessageIDs) Consume(consumer func(bucketMessageID *BucketMessageID)) (consumed bool) {
	for _, cachedBucketMessageID := range cachedBucketMessageIDs {
		consumed = cachedBucketMessageID.Consume(func(output *BucketMessageID) {
			consumer(output)
		}) || consumed
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
