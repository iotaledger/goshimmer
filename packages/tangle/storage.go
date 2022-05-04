package tangle

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

const (
	// PrefixMessage defines the storage prefix for message.
	PrefixMessage byte = iota

	// PrefixMessageMetadata defines the storage prefix for message metadata.
	PrefixMessageMetadata

	// PrefixApprovers defines the storage prefix for approvers.
	PrefixApprovers

	// PrefixMissingMessage defines the storage prefix for missing message.
	PrefixMissingMessage

	// PrefixAttachments defines the storage prefix for attachments.
	PrefixAttachments

	// PrefixMarkerBranchIDMapping defines the storage prefix for the PrefixMarkerBranchIDMapping.
	PrefixMarkerBranchIDMapping

	// PrefixBranchVoters defines the storage prefix for the BranchVoters.
	PrefixBranchVoters

	// PrefixLatestBranchVotes defines the storage prefix for the LatestBranchVotes.
	PrefixLatestBranchVotes

	// PrefixLatestMarkerVotes defines the storage prefix for the LatestMarkerVotes.
	PrefixLatestMarkerVotes

	// PrefixBranchWeight defines the storage prefix for the BranchWeight.
	PrefixBranchWeight

	// PrefixMarkerMessageMapping defines the storage prefix for the MarkerMessageMapping.
	PrefixMarkerMessageMapping

	// DBSequenceNumber defines the db sequence number.
	DBSequenceNumber = "seq"

	// cacheTime defines the number of seconds an object will wait in storage cache.
	cacheTime = 2 * time.Second

	// approvalWeightCacheTime defines the number of seconds an object related to approval weight will wait in storage cache.
	approvalWeightCacheTime = 20 * time.Second
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage represents the storage of messages.
type Storage struct {
	tangle                            *Tangle
	messageStorage                    *objectstorage.ObjectStorage[*Message]
	messageMetadataStorage            *objectstorage.ObjectStorage[*MessageMetadata]
	approverStorage                   *objectstorage.ObjectStorage[*Approver]
	missingMessageStorage             *objectstorage.ObjectStorage[*MissingMessage]
	attachmentStorage                 *objectstorage.ObjectStorage[*Attachment]
	markerIndexBranchIDMappingStorage *objectstorage.ObjectStorage[*MarkerIndexBranchIDMapping]
	branchVotersStorage               *objectstorage.ObjectStorage[*BranchVoters]
	latestBranchVotesStorage          *objectstorage.ObjectStorage[*LatestBranchVotes]
	latestMarkerVotesStorage          *objectstorage.ObjectStorage[*LatestMarkerVotes]
	branchWeightStorage               *objectstorage.ObjectStorage[*BranchWeight]
	markerMessageMappingStorage       *objectstorage.ObjectStorage[*MarkerMessageMapping]

	Events   *StorageEvents
	shutdown chan struct{}
}

// NewStorage creates a new Storage.
func NewStorage(tangle *Tangle) (storage *Storage) {
	cacheProvider := tangle.Options.CacheTimeProvider

	storage = &Storage{
		tangle:                            tangle,
		shutdown:                          make(chan struct{}),
		messageStorage:                    objectstorage.New[*Message](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMessage), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		messageMetadataStorage:            objectstorage.New[*MessageMetadata](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMessageMetadata), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:                   objectstorage.New[*Approver](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixApprovers), cacheProvider.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, ApproverTypeLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		missingMessageStorage:             objectstorage.New[*MissingMessage](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMissingMessage), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		attachmentStorage:                 objectstorage.New[*Attachment](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixAttachments), cacheProvider.CacheTime(cacheTime), objectstorage.PartitionKey(ledgerstate.TransactionIDLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		markerIndexBranchIDMappingStorage: objectstorage.New[*MarkerIndexBranchIDMapping](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMarkerBranchIDMapping), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		branchVotersStorage:               objectstorage.New[*BranchVoters](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixBranchVoters), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		latestBranchVotesStorage:          objectstorage.New[*LatestBranchVotes](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixLatestBranchVotes), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		latestMarkerVotesStorage:          objectstorage.New[*LatestMarkerVotes](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixLatestMarkerVotes), cacheProvider.CacheTime(approvalWeightCacheTime), LatestMarkerVotesKeyPartition, objectstorage.LeakDetectionEnabled(false)),
		branchWeightStorage:               objectstorage.New[*BranchWeight](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixBranchWeight), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		markerMessageMappingStorage:       objectstorage.New[*MarkerMessageMapping](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMarkerMessageMapping), cacheProvider.CacheTime(cacheTime), MarkerMessageMappingPartitionKeys, objectstorage.StoreOnCreation(true)),

		Events: &StorageEvents{
			MessageStored:        events.NewEvent(MessageIDCaller),
			MessageRemoved:       events.NewEvent(MessageIDCaller),
			MissingMessageStored: events.NewEvent(MessageIDCaller),
		},
	}

	storage.storeGenesis()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (s *Storage) Setup() {
	s.tangle.Parser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		s.tangle.Storage.StoreMessage(msgParsedEvent.Message)
	}))
}

// StoreMessage stores a new message to the message store.
func (s *Storage) StoreMessage(message *Message) {
	// retrieve MessageID
	messageID := message.ID()

	// store Messages only once by using the existence of the Metadata as a guard
	storedMetadata, stored := s.messageMetadataStorage.StoreIfAbsent(NewMessageMetadata(messageID))
	if !stored {
		return
	}

	// create typed version of the stored MessageMetadata
	cachedMsgMetadata := storedMetadata
	defer cachedMsgMetadata.Release()

	// store Message
	cachedMessage := s.messageStorage.Store(message)
	defer cachedMessage.Release()

	message.ForEachParent(func(parent Parent) {
		s.approverStorage.Store(NewApprover(ParentTypeToApproverType[parent.Type], parent.ID, messageID)).Release()
	})

	// trigger events
	if s.missingMessageStorage.DeleteIfPresent(messageID.Bytes()) {
		s.tangle.Storage.Events.MissingMessageStored.Trigger(messageID)
	}

	// messages are stored, trigger MessageStored event to move on next check
	s.Events.MessageStored.Trigger(message.ID())
}

// Message retrieves a message from the message store.
func (s *Storage) Message(messageID MessageID) *objectstorage.CachedObject[*Message] {
	return s.messageStorage.Load(messageID[:])
}

// MessageMetadata retrieves the MessageMetadata with the given MessageID.
func (s *Storage) MessageMetadata(messageID MessageID, computeIfAbsentCallback ...func() *MessageMetadata) *objectstorage.CachedObject[*MessageMetadata] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) *MessageMetadata {
			return computeIfAbsentCallback[0]()
		})
	}

	return s.messageMetadataStorage.Load(messageID[:])
}

// Approvers retrieves the Approvers of a Message from the object storage. It is possible to provide an optional
// ApproverType to only return the corresponding Approvers.
func (s *Storage) Approvers(messageID MessageID, optionalApproverType ...ApproverType) (cachedApprovers objectstorage.CachedObjects[*Approver]) {
	var iterationPrefix []byte
	if len(optionalApproverType) >= 1 {
		iterationPrefix = byteutils.ConcatBytes(messageID.Bytes(), optionalApproverType[0].Bytes())
	} else {
		iterationPrefix = messageID.Bytes()
	}

	cachedApprovers = make(objectstorage.CachedObjects[*Approver], 0)
	s.approverStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Approver]) bool {
		cachedApprovers = append(cachedApprovers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(iterationPrefix))

	return
}

// StoreMissingMessage stores a new MissingMessage entry in the object storage.
func (s *Storage) StoreMissingMessage(missingMessage *MissingMessage) (cachedMissingMessage *objectstorage.CachedObject[*MissingMessage], stored bool) {
	cachedObject, stored := s.missingMessageStorage.StoreIfAbsent(missingMessage)
	cachedMissingMessage = cachedObject

	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (s *Storage) MissingMessages() (ids []MessageID) {
	s.missingMessageStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MissingMessage]) bool {
		cachedObject.Consume(func(object *MissingMessage) {
			ids = append(ids, object.MessageID())
		})

		return true
	})
	return
}

// StoreAttachment stores a new attachment if not already stored.
func (s *Storage) StoreAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) (cachedAttachment *objectstorage.CachedObject[*Attachment], stored bool) {
	cachedAttachment, stored = s.attachmentStorage.StoreIfAbsent(NewAttachment(transactionID, messageID))
	if !stored {
		return
	}
	return
}

// Attachments retrieves the attachment of a transaction in attachmentStorage.
func (s *Storage) Attachments(transactionID ledgerstate.TransactionID) (cachedAttachments objectstorage.CachedObjects[*Attachment]) {
	s.attachmentStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Attachment]) bool {
		cachedAttachments = append(cachedAttachments, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(transactionID.Bytes()))
	return
}

// AttachmentMessageIDs returns the messageIDs of the transaction in attachmentStorage.
func (s *Storage) AttachmentMessageIDs(transactionID ledgerstate.TransactionID) (messageIDs MessageIDs) {
	messageIDs = NewMessageIDs()
	s.Attachments(transactionID).Consume(func(attachment *Attachment) {
		messageIDs.Add(attachment.MessageID())
	})
	return
}

// IsTransactionAttachedByMessage checks whether Transaction with transactionID is attached by Message with messageID.
func (s *Storage) IsTransactionAttachedByMessage(transactionID ledgerstate.TransactionID, messageID MessageID) (attached bool) {
	return s.attachmentStorage.Contains(NewAttachment(transactionID, messageID).ObjectStorageKey())
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (s *Storage) DeleteMessage(messageID MessageID) {
	s.Message(messageID).Consume(func(currentMsg *Message) {
		currentMsg.ForEachParent(func(parent Parent) {
			s.deleteApprover(parent, messageID)
		})

		s.messageMetadataStorage.Delete(messageID[:])
		s.messageStorage.Delete(messageID[:])

		s.Events.MessageRemoved.Trigger(messageID)
	})
}

// DeleteMissingMessage deletes a message from the missingMessageStorage.
func (s *Storage) DeleteMissingMessage(messageID MessageID) {
	s.missingMessageStorage.Delete(messageID[:])
}

// MarkerIndexBranchIDMapping retrieves the MarkerIndexBranchIDMapping for the given SequenceID. It accepts an optional
// computeIfAbsent callback that can be used to dynamically create a MarkerIndexBranchIDMapping if it doesn't exist,
// yet.
func (s *Storage) MarkerIndexBranchIDMapping(sequenceID markers.SequenceID, computeIfAbsentCallback ...func(sequenceID markers.SequenceID) *MarkerIndexBranchIDMapping) *objectstorage.CachedObject[*MarkerIndexBranchIDMapping] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.markerIndexBranchIDMappingStorage.ComputeIfAbsent(sequenceID.Bytes(), func(key []byte) *MarkerIndexBranchIDMapping {
			return computeIfAbsentCallback[0](sequenceID)
		})
	}

	return s.markerIndexBranchIDMappingStorage.Load(sequenceID.Bytes())
}

// StoreMarkerMessageMapping stores a MarkerMessageMapping in the underlying object storage.
func (s *Storage) StoreMarkerMessageMapping(markerMessageMapping *MarkerMessageMapping) {
	s.markerMessageMappingStorage.Store(markerMessageMapping).Release()
}

// DeleteMarkerMessageMapping deleted a MarkerMessageMapping in the underlying object storage.
func (s *Storage) DeleteMarkerMessageMapping(branchID ledgerstate.BranchID, messageID MessageID) {
	s.markerMessageMappingStorage.Delete(byteutils.ConcatBytes(branchID.Bytes(), messageID.Bytes()))
}

// MarkerMessageMapping retrieves the MarkerMessageMapping associated with the given details.
func (s *Storage) MarkerMessageMapping(marker *markers.Marker) (cachedMarkerMessageMappings *objectstorage.CachedObject[*MarkerMessageMapping]) {
	return s.markerMessageMappingStorage.Load(marker.Bytes())
}

// MarkerMessageMappings retrieves the MarkerMessageMappings of a Sequence in the object storage.
func (s *Storage) MarkerMessageMappings(sequenceID markers.SequenceID) (cachedMarkerMessageMappings objectstorage.CachedObjects[*MarkerMessageMapping]) {
	s.markerMessageMappingStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MarkerMessageMapping]) bool {
		cachedMarkerMessageMappings = append(cachedMarkerMessageMappings, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(sequenceID.Bytes()))
	return
}

// BranchVoters retrieves the BranchVoters with the given ledgerstate.BranchID.
func (s *Storage) BranchVoters(branchID ledgerstate.BranchID, computeIfAbsentCallback ...func(branchID ledgerstate.BranchID) *BranchVoters) *objectstorage.CachedObject[*BranchVoters] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchVotersStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *BranchVoters {
			return computeIfAbsentCallback[0](branchID)
		})
	}

	return s.branchVotersStorage.Load(branchID.Bytes())
}

// LatestBranchVotes retrieves the LatestBranchVotes of the given Voter.
func (s *Storage) LatestBranchVotes(voter Voter, computeIfAbsentCallback ...func(voter Voter) *LatestBranchVotes) *objectstorage.CachedObject[*LatestBranchVotes] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.latestBranchVotesStorage.ComputeIfAbsent(byteutils.ConcatBytes(voter.Bytes()), func(key []byte) *LatestBranchVotes {
			return computeIfAbsentCallback[0](voter)
		})
	}

	return s.latestBranchVotesStorage.Load(byteutils.ConcatBytes(voter.Bytes()))
}

// LatestMarkerVotes retrieves the LatestMarkerVotes of the given voter for the named Sequence.
func (s *Storage) LatestMarkerVotes(sequenceID markers.SequenceID, voter Voter, computeIfAbsentCallback ...func(sequenceID markers.SequenceID, voter Voter) *LatestMarkerVotes) *objectstorage.CachedObject[*LatestMarkerVotes] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.latestMarkerVotesStorage.ComputeIfAbsent(byteutils.ConcatBytes(sequenceID.Bytes(), voter.Bytes()), func(key []byte) *LatestMarkerVotes {
			return computeIfAbsentCallback[0](sequenceID, voter)
		})
	}

	return s.latestMarkerVotesStorage.Load(byteutils.ConcatBytes(sequenceID.Bytes(), voter.Bytes()))
}

// AllLatestMarkerVotes retrieves all LatestMarkerVotes for the named Sequence.
func (s *Storage) AllLatestMarkerVotes(sequenceID markers.SequenceID) (cachedLatestMarkerVotesByVoter CachedLatestMarkerVotesByVoter) {
	cachedLatestMarkerVotesByVoter = make(CachedLatestMarkerVotesByVoter)

	s.latestMarkerVotesStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*LatestMarkerVotes]) bool {
		cachedLatestMarkerVotes := cachedObject
		latestMarkerVotes, _ := cachedLatestMarkerVotes.Unwrap()
		cachedLatestMarkerVotesByVoter[latestMarkerVotes.Voter()] = cachedLatestMarkerVotes

		return true
	}, objectstorage.WithIteratorPrefix(sequenceID.Bytes()))

	return cachedLatestMarkerVotesByVoter
}

// BranchWeight retrieves the BranchWeight with the given BranchID.
func (s *Storage) BranchWeight(branchID ledgerstate.BranchID, computeIfAbsentCallback ...func(branchID ledgerstate.BranchID) *BranchWeight) *objectstorage.CachedObject[*BranchWeight] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchWeightStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *BranchWeight {
			return computeIfAbsentCallback[0](branchID)
		})
	}

	return s.branchWeightStorage.Load(branchID.Bytes())
}

func (s *Storage) storeGenesis() {
	s.MessageMetadata(EmptyMessageID, func() *MessageMetadata {
		genesisMetadata := &MessageMetadata{
			messageMetadataInner{
				SolidificationTime: clock.SyncedTime().Add(time.Duration(-20) * time.Minute),
				MessageID:          EmptyMessageID,
				Solid:              true,
				StructureDetails: &markers.StructureDetails{
					Rank:          0,
					IsPastMarker:  false,
					PastMarkers:   markers.NewMarkers(),
					FutureMarkers: markers.NewMarkers(),
				},
				Scheduled: true,
				Booked:    true,
			},
		}

		genesisMetadata.Persist()
		genesisMetadata.SetModified()

		return genesisMetadata
	}).Release()
}

// deleteApprover deletes the Approver from the object storage that was created by the specified parent.
func (s *Storage) deleteApprover(parent Parent, approvingMessage MessageID) {
	s.approverStorage.Delete(byteutils.ConcatBytes(parent.ID.Bytes(), ParentTypeToApproverType[parent.Type].Bytes(), approvingMessage.Bytes()))
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (s *Storage) Shutdown() {
	s.messageStorage.Shutdown()
	s.messageMetadataStorage.Shutdown()
	s.approverStorage.Shutdown()
	s.missingMessageStorage.Shutdown()
	s.attachmentStorage.Shutdown()
	s.markerIndexBranchIDMappingStorage.Shutdown()
	s.branchVotersStorage.Shutdown()
	s.latestBranchVotesStorage.Shutdown()
	s.latestMarkerVotesStorage.Shutdown()
	s.branchWeightStorage.Shutdown()
	s.markerMessageMappingStorage.Shutdown()

	close(s.shutdown)
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
func (s *Storage) Prune() error {
	for _, storagePrune := range []func() error{
		s.messageStorage.Prune,
		s.messageMetadataStorage.Prune,
		s.approverStorage.Prune,
		s.missingMessageStorage.Prune,
		s.attachmentStorage.Prune,
		s.markerIndexBranchIDMappingStorage.Prune,
		s.branchVotersStorage.Prune,
		s.latestBranchVotesStorage.Prune,
		s.latestMarkerVotesStorage.Prune,
		s.branchWeightStorage.Prune,
		s.markerMessageMappingStorage.Prune,
	} {
		if err := storagePrune(); err != nil {
			err = fmt.Errorf("failed to prune storage: %w", err)
			return err
		}
	}

	s.storeGenesis()

	return nil
}

// DBStatsResult is a structure containing all the statistics retrieved by DBStats() method.
type DBStatsResult struct {
	StoredCount                   int
	SolidCount                    int
	BookedCount                   int
	ScheduledCount                int
	SumSolidificationReceivedTime time.Duration
	SumBookedReceivedTime         time.Duration
	SumSchedulerReceivedTime      time.Duration
	SumSchedulerBookedTime        time.Duration
	MissingMessageCount           int
}

// DBStats returns the number of solid messages and total number of messages in the database (messageMetadataStorage,
// that should contain the messages as messageStorage), the number of messages in missingMessageStorage, furthermore
// the average time it takes to solidify messages.
func (s *Storage) DBStats() (res DBStatsResult) {
	s.messageMetadataStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MessageMetadata]) bool {
		cachedObject.Consume(func(msgMetaData *MessageMetadata) {
			res.StoredCount++
			received := msgMetaData.ReceivedTime()
			if msgMetaData.IsSolid() {
				res.SolidCount++
				res.SumSolidificationReceivedTime += msgMetaData.SolidificationTime().Sub(received)
			}
			if msgMetaData.IsBooked() {
				res.BookedCount++
				res.SumBookedReceivedTime += msgMetaData.BookedTime().Sub(received)
			}
			if msgMetaData.Scheduled() {
				res.ScheduledCount++
				res.SumSchedulerReceivedTime += msgMetaData.ScheduledTime().Sub(received)
				res.SumSchedulerBookedTime += msgMetaData.ScheduledTime().Sub(msgMetaData.messageMetadataInner.BookedTime)
			}
		})
		return true
	})

	s.missingMessageStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MissingMessage]) bool {
		cachedObject.Consume(func(object *MissingMessage) {
			res.MissingMessageCount++
		})
		return true
	})
	return
}

// RetrieveAllTips returns the tips (i.e., solid messages that are not part of the approvers list).
// It iterates over the messageMetadataStorage, thus only use this method if necessary.
// TODO: improve this function.
func (s *Storage) RetrieveAllTips() (tips []MessageID) {
	s.messageMetadataStorage.ForEach(func(key []byte, cachedMessage *objectstorage.CachedObject[*MessageMetadata]) bool {
		cachedMessage.Consume(func(messageMetadata *MessageMetadata) {
			if messageMetadata != nil && messageMetadata.IsSolid() {
				cachedApprovers := s.Approvers(messageMetadata.messageMetadataInner.MessageID)
				if len(cachedApprovers) == 0 {
					tips = append(tips, messageMetadata.messageMetadataInner.MessageID)
				}
				cachedApprovers.Release()
			}
		})
		return true
	})
	return tips
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorageEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// StorageEvents represents events happening on the message store.
type StorageEvents struct {
	// Fired when a message has been stored.
	MessageStored *events.Event

	// Fired when a message was removed from storage.
	MessageRemoved *events.Event

	// Fired when a message which was previously marked as missing was received.
	MissingMessageStored *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApproverType /////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// StrongApprover is the ApproverType that represents references formed by strong parents.
	StrongApprover ApproverType = iota

	// WeakApprover is the ApproverType that represents references formed by weak parents.
	WeakApprover

	// ShallowLikeApprover is the ApproverType that represents references formed by shallow like parents.
	ShallowLikeApprover

	// ShallowDislikeApprover is the ApproverType that represents references formed by shallow dislike parents.
	ShallowDislikeApprover
)

// ApproverTypeLength contains the amount of bytes that a marshaled version of the ApproverType contains.
const ApproverTypeLength = 1

// ApproverType is a type that represents the different kind of reverse mapping that we have for references formed by
// strong and weak parents.
type ApproverType uint8

// ParentTypeToApproverType represents a convenient mapping between a parent type and the approver type.
var ParentTypeToApproverType = map[ParentsType]ApproverType{
	StrongParentType:         StrongApprover,
	WeakParentType:           WeakApprover,
	ShallowLikeParentType:    ShallowLikeApprover,
	ShallowDislikeParentType: ShallowDislikeApprover,
}

// Bytes returns a marshaled version of the ApproverType.
func (a ApproverType) Bytes() []byte {
	return []byte{byte(a)}
}

// String returns a human readable version of the ApproverType.
func (a ApproverType) String() string {
	switch a {
	case StrongApprover:
		return "ApproverType(StrongApprover)"
	case WeakApprover:
		return "ApproverType(WeakApprover)"
	case ShallowLikeApprover:
		return "ApproverType(ShallowLikeApprover)"
	case ShallowDislikeApprover:
		return "ApproverType(ShallowDislikeApprover)"
	default:
		return fmt.Sprintf("ApproverType(%X)", uint8(a))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Approver /////////////////////////////////////////////////////////////////////////////////////////////////////

// Approver is an approver of a given referenced message.
type Approver struct {
	approverInner `serix:"0"`
}

type approverInner struct {
	// the message which got referenced by the approver message.
	ReferencedMessageID MessageID `serix:"0"`

	// ApproverType defines if the reference was created by a strong, weak, shallowlike or shallowdislike parent reference.
	ApproverType ApproverType `serix:"1"`

	// the message which approved/referenced the given referenced message.
	ApproverMessageID MessageID `serix:"2"`

	objectstorage.StorableObjectFlags
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(approverType ApproverType, referencedMessageID MessageID, approverMessageID MessageID) *Approver {
	approver := &Approver{
		approverInner{
			ApproverType:        approverType,
			ReferencedMessageID: referencedMessageID,
			ApproverMessageID:   approverMessageID,
		},
	}
	return approver
}

// FromObjectStorage creates an Approver from sequences of key and bytes.
func (a *Approver) FromObjectStorage(key, _ []byte) (objectstorage.StorableObject, error) {
	result, err := a.FromBytes(key)
	if err != nil {
		err = errors.Errorf("failed to parse Approver from bytes: %w", err)
	}
	return result, err
}

// FromBytes parses the given bytes into an approver.
func (a *Approver) FromBytes(data []byte) (result *Approver, err error) {
	approver := new(Approver)
	if a != nil {
		approver = a
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data, approver, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Approver: %w", err)
		return approver, err
	}
	return approver, err
}

// Type returns the type of the Approver reference.
func (a *Approver) Type() ApproverType {
	return a.approverInner.ApproverType
}

// ReferencedMessageID returns the ID of the message which is referenced by the approver.
func (a *Approver) ReferencedMessageID() MessageID {
	return a.approverInner.ReferencedMessageID
}

// ApproverMessageID returns the ID of the message which referenced the given approved message.
func (a *Approver) ApproverMessageID() MessageID {
	return a.approverInner.ApproverMessageID
}

// Bytes returns the bytes of the approver.
func (a *Approver) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns the string representation of the approver.
func (a *Approver) String() string {
	return stringify.Struct("Approver",
		stringify.StructField("referencedMessageID", a.ReferencedMessageID()),
		stringify.StructField("approverMessageID", a.ApproverMessageID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *Approver) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue returns the value of the stored approver object.
func (a *Approver) ObjectStorageValue() (result []byte) {
	return
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ objectstorage.StorableObject = new(Approver)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Attachment ///////////////////////////////////////////////////////////////////////////////////////////////////

// Attachment stores the information which transaction was attached by which message. We need this to be able to perform
// reverse lookups from transactions to their corresponding messages that attach them.
type Attachment struct {
	attachmentInner `serix:"0"`
}
type attachmentInner struct {
	TransactionID ledgerstate.TransactionID `serix:"0"`
	MessageID     MessageID                 `serix:"1"`

	objectstorage.StorableObjectFlags
}

// NewAttachment creates an attachment object with the given information.
func NewAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) *Attachment {
	return &Attachment{
		attachmentInner{
			TransactionID: transactionID,
			MessageID:     messageID,
		},
	}
}

// FromObjectStorage creates an Attachment from sequences of key and bytes.
func (a *Attachment) FromObjectStorage(key, _ []byte) (objectstorage.StorableObject, error) {
	result, err := a.FromBytes(key)
	if err != nil {
		err = errors.Errorf("failed to parse attachment from object storage: %w", err)
	}
	return result, err
}

// FromBytes unmarshals an Attachment from a sequence of bytes - it either creates a new object or fills the
// optionally provided one with the parsed information.
func (a *Attachment) FromBytes(data []byte) (result *Attachment, err error) {
	attachment := new(Attachment)
	if a != nil {
		attachment = a
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data, attachment, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Attachment: %w", err)
		return attachment, err
	}
	return attachment, err
}

// TransactionID returns the transactionID of this Attachment.
func (a *Attachment) TransactionID() ledgerstate.TransactionID {
	return a.attachmentInner.TransactionID
}

// MessageID returns the messageID of this Attachment.
func (a *Attachment) MessageID() MessageID {
	return a.attachmentInner.MessageID
}

// Bytes marshals the Attachment into a sequence of bytes.
func (a *Attachment) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns a human readable version of the Attachment.
func (a *Attachment) String() string {
	return stringify.Struct("Attachment",
		stringify.StructField("transactionId", a.TransactionID()),
		stringify.StructField("messageID", a.MessageID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *Attachment) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the "content part" of an Attachment to a sequence of bytes. Since all of the information
// for this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (a *Attachment) ObjectStorageValue() (data []byte) {
	return
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = new(Attachment)

// AttachmentLength holds the length of a marshaled Attachment in bytes.
const AttachmentLength = ledgerstate.TransactionIDLength + MessageIDLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MissingMessage ///////////////////////////////////////////////////////////////////////////////////////////////

// MissingMessage represents a missing message.
type MissingMessage struct {
	missingMessageInner `serix:"0"`
}

type missingMessageInner struct {
	MessageID    MessageID
	MissingSince time.Time `serix:"0"`

	objectstorage.StorableObjectFlags
}

// NewMissingMessage creates new missing message with the specified messageID.
func NewMissingMessage(messageID MessageID) *MissingMessage {
	return &MissingMessage{
		missingMessageInner{
			MessageID:    messageID,
			MissingSince: time.Now(),
		},
	}
}

// FromObjectStorage creates an MissingMessage from sequences of key and bytes.
func (m *MissingMessage) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := m.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = fmt.Errorf("failed to parse missing message from object storage: %w", err)
	}
	return result, err
}

// FromBytes parses the given bytes into a MissingMessage.
func (m *MissingMessage) FromBytes(data []byte) (result *MissingMessage, err error) {
	missingMsg := new(MissingMessage)
	if m != nil {
		missingMsg = m
	}
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, &missingMsg.missingMessageInner.MessageID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse MissingMessage.MessageID: %w", err)
		return missingMsg, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], missingMsg, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse MissingMessage: %w", err)
		return missingMsg, err
	}
	return missingMsg, err
}

// MessageID returns the id of the message.
func (m *MissingMessage) MessageID() MessageID {
	return m.missingMessageInner.MessageID
}

// MissingSince returns the time since when this message is missing.
func (m *MissingMessage) MissingSince() time.Time {
	return m.missingMessageInner.MissingSince
}

// Bytes returns a marshaled version of this MissingMessage.
func (m *MissingMessage) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MissingMessage) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m.missingMessageInner.MessageID, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the MissingMessage into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (m *MissingMessage) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = new(MissingMessage)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
