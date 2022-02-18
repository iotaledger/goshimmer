package tangle

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	genericobjectstorage "github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
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
	messageStorage                    *genericobjectstorage.ObjectStorage[*Message]
	messageMetadataStorage            *genericobjectstorage.ObjectStorage[*MessageMetadata]
	approverStorage                   *genericobjectstorage.ObjectStorage[*Approver]
	missingMessageStorage             *genericobjectstorage.ObjectStorage[*MissingMessage]
	attachmentStorage                 *genericobjectstorage.ObjectStorage[*Attachment]
	markerIndexBranchIDMappingStorage *genericobjectstorage.ObjectStorage[*MarkerIndexBranchIDMapping]
	branchVotersStorage               *genericobjectstorage.ObjectStorage[*BranchVoters]
	latestBranchVotesStorage          *genericobjectstorage.ObjectStorage[*LatestBranchVotes]
	latestMarkerVotesStorage          *genericobjectstorage.ObjectStorage[*LatestMarkerVotes]
	branchWeightStorage               *genericobjectstorage.ObjectStorage[*BranchWeight]
	markerMessageMappingStorage       *genericobjectstorage.ObjectStorage[*MarkerMessageMapping]

	Events   *StorageEvents
	shutdown chan struct{}
}

// NewStorage creates a new Storage.
func NewStorage(tangle *Tangle) (storage *Storage) {
	cacheProvider := tangle.Options.CacheTimeProvider

	storage = &Storage{
		tangle:                            tangle,
		shutdown:                          make(chan struct{}),
		messageStorage:                    genericobjectstorage.New[*Message](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixMessage}), cacheProvider.CacheTime(cacheTime), genericobjectstorage.LeakDetectionEnabled(false), genericobjectstorage.StoreOnCreation(true)),
		messageMetadataStorage:            genericobjectstorage.New[*MessageMetadata](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixMessageMetadata}), cacheProvider.CacheTime(cacheTime), genericobjectstorage.LeakDetectionEnabled(false)),
		approverStorage:                   genericobjectstorage.New[*Approver](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixApprovers}), cacheProvider.CacheTime(cacheTime), genericobjectstorage.PartitionKey(MessageIDLength, ApproverTypeLength, MessageIDLength), genericobjectstorage.LeakDetectionEnabled(false), genericobjectstorage.StoreOnCreation(true)),
		missingMessageStorage:             genericobjectstorage.New[*MissingMessage](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixMissingMessage}), cacheProvider.CacheTime(cacheTime), genericobjectstorage.LeakDetectionEnabled(false), genericobjectstorage.StoreOnCreation(true)),
		attachmentStorage:                 genericobjectstorage.New[*Attachment](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixAttachments}), cacheProvider.CacheTime(cacheTime), genericobjectstorage.PartitionKey(ledgerstate.TransactionIDLength, MessageIDLength), genericobjectstorage.LeakDetectionEnabled(false), genericobjectstorage.StoreOnCreation(true)),
		markerIndexBranchIDMappingStorage: genericobjectstorage.New[*MarkerIndexBranchIDMapping](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixMarkerBranchIDMapping}), cacheProvider.CacheTime(cacheTime), genericobjectstorage.LeakDetectionEnabled(false)),
		branchVotersStorage:               genericobjectstorage.New[*BranchVoters](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixBranchVoters}), cacheProvider.CacheTime(approvalWeightCacheTime), genericobjectstorage.LeakDetectionEnabled(false)),
		latestBranchVotesStorage:          genericobjectstorage.New[*LatestBranchVotes](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixLatestBranchVotes}), cacheProvider.CacheTime(approvalWeightCacheTime), genericobjectstorage.LeakDetectionEnabled(false)),
		latestMarkerVotesStorage:          genericobjectstorage.New[*LatestMarkerVotes](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixLatestMarkerVotes}), cacheProvider.CacheTime(approvalWeightCacheTime), LatestMarkerVotesKeyPartition, genericobjectstorage.LeakDetectionEnabled(false)),
		branchWeightStorage:               genericobjectstorage.New[*BranchWeight](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixBranchWeight}), cacheProvider.CacheTime(approvalWeightCacheTime), genericobjectstorage.LeakDetectionEnabled(false)),
		markerMessageMappingStorage:       genericobjectstorage.New[*MarkerMessageMapping](tangle.Options.Store.WithRealm([]byte{database.PrefixTangle, PrefixMarkerMessageMapping}), cacheProvider.CacheTime(cacheTime), MarkerMessageMappingPartitionKeys, genericobjectstorage.StoreOnCreation(true)),

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

	// store approvers
	message.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) bool {
		s.approverStorage.Store(NewApprover(StrongApprover, parentMessageID, messageID)).Release()

		return true
	})
	for _, parentType := range []ParentsType{ShallowLikeParentType, ShallowDislikeParentType, WeakParentType} {
		message.ForEachParentByType(parentType, func(parentMessageID MessageID) bool {
			if cachedObject, likeStored := s.approverStorage.StoreIfAbsent(NewApprover(WeakApprover, parentMessageID, messageID)); likeStored {
				cachedObject.Release()
			}

			return true
		})
	}

	// trigger events
	if s.missingMessageStorage.DeleteIfPresent(messageID[:]) {
		s.tangle.Storage.Events.MissingMessageStored.Trigger(messageID)
	}

	// messages are stored, trigger MessageStored event to move on next check
	s.Events.MessageStored.Trigger(message.ID())
}

// Message retrieves a message from the message store.
func (s *Storage) Message(messageID MessageID) *genericobjectstorage.CachedObject[*Message] {
	return s.messageStorage.Load(messageID[:])
}

// MessageMetadata retrieves the MessageMetadata with the given MessageID.
func (s *Storage) MessageMetadata(messageID MessageID, computeIfAbsentCallback ...func() *MessageMetadata) *genericobjectstorage.CachedObject[*MessageMetadata] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) *MessageMetadata {
			return computeIfAbsentCallback[0]()
		})
	}

	return s.messageMetadataStorage.Load(messageID[:])
}

// Approvers retrieves the Approvers of a Message from the object storage. It is possible to provide an optional
// ApproverType to only return the corresponding Approvers.
func (s *Storage) Approvers(messageID MessageID, optionalApproverType ...ApproverType) (cachedApprovers genericobjectstorage.CachedObjects[*Approver]) {
	var iterationPrefix []byte
	if len(optionalApproverType) >= 1 {
		iterationPrefix = byteutils.ConcatBytes(messageID.Bytes(), optionalApproverType[0].Bytes())
	} else {
		iterationPrefix = messageID.Bytes()
	}

	cachedApprovers = make(genericobjectstorage.CachedObjects[*Approver], 0)
	s.approverStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*Approver]) bool {
		cachedApprovers = append(cachedApprovers, cachedObject)
		return true
	}, genericobjectstorage.WithIteratorPrefix(iterationPrefix))

	return
}

// StoreMissingMessage stores a new MissingMessage entry in the object storage.
func (s *Storage) StoreMissingMessage(missingMessage *MissingMessage) (cachedMissingMessage *genericobjectstorage.CachedObject[*MissingMessage], stored bool) {
	cachedObject, stored := s.missingMessageStorage.StoreIfAbsent(missingMessage)
	cachedMissingMessage = cachedObject

	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (s *Storage) MissingMessages() (ids []MessageID) {
	s.missingMessageStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*MissingMessage]) bool {
		cachedObject.Consume(func(object *MissingMessage) {
			ids = append(ids, object.messageID)
		})

		return true
	})
	return
}

// StoreAttachment stores a new attachment if not already stored.
func (s *Storage) StoreAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) (cachedAttachment *genericobjectstorage.CachedObject[*Attachment], stored bool) {
	cachedAttachment, stored = s.attachmentStorage.StoreIfAbsent(NewAttachment(transactionID, messageID))
	if !stored {
		return
	}
	return
}

// Attachments retrieves the attachment of a transaction in attachmentStorage.
func (s *Storage) Attachments(transactionID ledgerstate.TransactionID) (cachedAttachments genericobjectstorage.CachedObjects[*Attachment]) {
	s.attachmentStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*Attachment]) bool {
		cachedAttachments = append(cachedAttachments, cachedObject)
		return true
	}, genericobjectstorage.WithIteratorPrefix(transactionID.Bytes()))
	return
}

// AttachmentMessageIDs returns the messageIDs of the transaction in attachmentStorage.
func (s *Storage) AttachmentMessageIDs(transactionID ledgerstate.TransactionID) (messageIDs MessageIDsSlice) {
	s.Attachments(transactionID).Consume(func(attachment *Attachment) {
		messageIDs = append(messageIDs, attachment.MessageID())
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
		currentMsg.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) bool {
			s.deleteStrongApprover(parentMessageID, messageID)

			return true
		})
		currentMsg.ForEachParentByType(WeakParentType, func(parentMessageID MessageID) bool {
			s.deleteWeakApprover(parentMessageID, messageID)

			return true
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
func (s *Storage) MarkerIndexBranchIDMapping(sequenceID markers.SequenceID, computeIfAbsentCallback ...func(sequenceID markers.SequenceID) *MarkerIndexBranchIDMapping) *genericobjectstorage.CachedObject[*MarkerIndexBranchIDMapping] {
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
func (s *Storage) MarkerMessageMapping(marker *markers.Marker) (cachedMarkerMessageMappings *genericobjectstorage.CachedObject[*MarkerMessageMapping]) {
	return s.markerMessageMappingStorage.Load(marker.Bytes())
}

// MarkerMessageMappings retrieves the MarkerMessageMappings of a Sequence in the object storage.
func (s *Storage) MarkerMessageMappings(sequenceID markers.SequenceID) (cachedMarkerMessageMappings genericobjectstorage.CachedObjects[*MarkerMessageMapping]) {
	s.markerMessageMappingStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*MarkerMessageMapping]) bool {
		cachedMarkerMessageMappings = append(cachedMarkerMessageMappings, cachedObject)
		return true
	}, genericobjectstorage.WithIteratorPrefix(sequenceID.Bytes()))
	return
}

// BranchVoters retrieves the BranchVoters with the given ledgerstate.BranchID.
func (s *Storage) BranchVoters(branchID ledgerstate.BranchID, computeIfAbsentCallback ...func(branchID ledgerstate.BranchID) *BranchVoters) *genericobjectstorage.CachedObject[*BranchVoters] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchVotersStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *BranchVoters {
			return computeIfAbsentCallback[0](branchID)
		})
	}

	return s.branchVotersStorage.Load(branchID.Bytes())
}

// LatestBranchVotes retrieves the LatestBranchVotes of the given Voter.
func (s *Storage) LatestBranchVotes(voter Voter, computeIfAbsentCallback ...func(voter Voter) *LatestBranchVotes) *genericobjectstorage.CachedObject[*LatestBranchVotes] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.latestBranchVotesStorage.ComputeIfAbsent(byteutils.ConcatBytes(voter.Bytes()), func(key []byte) *LatestBranchVotes {
			return computeIfAbsentCallback[0](voter)
		})
	}

	return s.latestBranchVotesStorage.Load(byteutils.ConcatBytes(voter.Bytes()))
}

// LatestMarkerVotes retrieves the LatestMarkerVotes of the given voter for the named Sequence.
func (s *Storage) LatestMarkerVotes(sequenceID markers.SequenceID, voter Voter, computeIfAbsentCallback ...func(sequenceID markers.SequenceID, voter Voter) *LatestMarkerVotes) *genericobjectstorage.CachedObject[*LatestMarkerVotes] {
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

	s.latestMarkerVotesStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*LatestMarkerVotes]) bool {
		cachedLatestMarkerVotes := cachedObject
		latestMarkerVotes, _ := cachedLatestMarkerVotes.Unwrap()
		cachedLatestMarkerVotesByVoter[latestMarkerVotes.Voter()] = cachedLatestMarkerVotes

		return true
	}, genericobjectstorage.WithIteratorPrefix(sequenceID.Bytes()))

	return cachedLatestMarkerVotesByVoter
}

// BranchWeight retrieves the BranchWeight with the given BranchID.
func (s *Storage) BranchWeight(branchID ledgerstate.BranchID, computeIfAbsentCallback ...func(branchID ledgerstate.BranchID) *BranchWeight) *genericobjectstorage.CachedObject[*BranchWeight] {
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
			solidificationTime: clock.SyncedTime().Add(time.Duration(-20) * time.Minute),
			messageID:          EmptyMessageID,
			solid:              true,
			structureDetails: &markers.StructureDetails{
				Rank:          0,
				IsPastMarker:  false,
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			},
			scheduled: true,
			booked:    true,
		}

		genesisMetadata.Persist()
		genesisMetadata.SetModified()

		return genesisMetadata
	}).Release()
}

// deleteStrongApprover deletes an Approver from the object storage that was created by a strong parent.
func (s *Storage) deleteStrongApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	s.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), StrongApprover.Bytes(), approvingMessage.Bytes()))
}

// deleteWeakApprover deletes an Approver from the object storage that was created by a weak parent.
func (s *Storage) deleteWeakApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	s.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), WeakApprover.Bytes(), approvingMessage.Bytes()))
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
	s.messageMetadataStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*MessageMetadata]) bool {
		cachedObject.Consume(func(object *MessageMetadata) {
			msgMetaData := object
			res.StoredCount++
			received := msgMetaData.ReceivedTime()
			if msgMetaData.IsSolid() {
				res.SolidCount++
				res.SumSolidificationReceivedTime += msgMetaData.solidificationTime.Sub(received)
			}
			if msgMetaData.IsBooked() {
				res.BookedCount++
				res.SumBookedReceivedTime += msgMetaData.bookedTime.Sub(received)
			}
			if msgMetaData.Scheduled() {
				res.ScheduledCount++
				res.SumSchedulerReceivedTime += msgMetaData.scheduledTime.Sub(received)
				res.SumSchedulerBookedTime += msgMetaData.scheduledTime.Sub(msgMetaData.bookedTime)
			}
		})
		return true
	})

	s.missingMessageStorage.ForEach(func(key []byte, cachedObject *genericobjectstorage.CachedObject[*MissingMessage]) bool {
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
	s.messageMetadataStorage.ForEach(func(key []byte, cachedMessage *genericobjectstorage.CachedObject[*MessageMetadata]) bool {
		cachedMessage.Consume(func(object *MessageMetadata) {
			messageMetadata := object
			if messageMetadata != nil && messageMetadata.IsSolid() {
				cachedApprovers := s.Approvers(messageMetadata.messageID)
				if len(cachedApprovers) == 0 {
					tips = append(tips, messageMetadata.messageID)
				}
				cachedApprovers.Consume(func(approver *Approver) {})
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
)

// ApproverTypeLength contains the amount of bytes that a marshaled version of the ApproverType contains.
const ApproverTypeLength = 1

// ApproverType is a type that represents the different kind of reverse mapping that we have for references formed by
// strong and weak parents.
type ApproverType uint8

// ApproverTypeFromBytes unmarshals an ApproverType from a sequence of bytes.
func ApproverTypeFromBytes(bytes []byte) (approverType ApproverType, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if approverType, err = ApproverTypeFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ApproverType from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ApproverTypeFromMarshalUtil unmarshals an ApproverType using a MarshalUtil (for easier unmarshaling).
func ApproverTypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (approverType ApproverType, err error) {
	untypedApproverType, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse ApproverType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if approverType = ApproverType(untypedApproverType); approverType != StrongApprover && approverType != WeakApprover {
		err = errors.Errorf("invalid ApproverType(%X): %w", approverType, cerrors.ErrParseBytesFailed)
		return
	}

	return
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
	default:
		return fmt.Sprintf("ApproverType(%X)", uint8(a))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Approver /////////////////////////////////////////////////////////////////////////////////////////////////////

// Approver is an approver of a given referenced message.
type Approver struct {
	// approverType defines if the reference was create by a strong or a weak parent reference.
	approverType ApproverType

	// the message which got referenced by the approver message.
	referencedMessageID MessageID

	// the message which approved/referenced the given referenced message.
	approverMessageID MessageID

	genericobjectstorage.StorableObjectFlags
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(approverType ApproverType, referencedMessageID MessageID, approverMessageID MessageID) *Approver {
	approver := &Approver{
		approverType:        approverType,
		referencedMessageID: referencedMessageID,
		approverMessageID:   approverMessageID,
	}
	return approver
}

// FromObjectStorage creates an Approver from sequences of key and bytes.
func (a *Approver) FromObjectStorage(key, bytes []byte) (genericobjectstorage.StorableObject, error) {
	return a.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes parses the given bytes into an approver.
func (*Approver) FromBytes(bytes []byte) (result genericobjectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ApproverFromMarshalUtil(marshalUtil)
	return
}

// ApproverFromMarshalUtil parses a new approver from the given marshal util.
func ApproverFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Approver, err error) {
	result = &Approver{}
	if result.referencedMessageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse referenced MessageID from MarshalUtil: %w", err)
		return
	}
	if result.approverType, err = ApproverTypeFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ApproverType from MarshalUtil: %w", err)
		return
	}
	if result.approverMessageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse approver MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// Type returns the type of the Approver reference.
func (a *Approver) Type() ApproverType {
	return a.approverType
}

// ReferencedMessageID returns the ID of the message which is referenced by the approver.
func (a *Approver) ReferencedMessageID() MessageID {
	return a.referencedMessageID
}

// ApproverMessageID returns the ID of the message which referenced the given approved message.
func (a *Approver) ApproverMessageID() MessageID {
	return a.approverMessageID
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

// ObjectStorageKey marshals the keys of the stored approver into a byte array.
// This includes the referencedMessageID and the approverMessageID.
func (a *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		Write(a.referencedMessageID).
		Write(a.approverType).
		Write(a.approverMessageID).
		Bytes()
}

// ObjectStorageValue returns the value of the stored approver object.
func (a *Approver) ObjectStorageValue() (result []byte) {
	return
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ genericobjectstorage.StorableObject = &Approver{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Attachment ///////////////////////////////////////////////////////////////////////////////////////////////////

// Attachment stores the information which transaction was attached by which message. We need this to be able to perform
// reverse lookups from transactions to their corresponding messages that attach them.
type Attachment struct {
	genericobjectstorage.StorableObjectFlags

	transactionID ledgerstate.TransactionID
	messageID     MessageID
}

// NewAttachment creates an attachment object with the given information.
func NewAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) *Attachment {
	return &Attachment{
		transactionID: transactionID,
		messageID:     messageID,
	}
}

// FromObjectStorage creates an Attachment from sequences of key and bytes.
func (a *Attachment) FromObjectStorage(key, bytes []byte) (genericobjectstorage.StorableObject, error) {
	return a.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes unmarshals an Attachment from a sequence of bytes - it either creates a new object or fills the
// optionally provided one with the parsed information.
func (*Attachment) FromBytes(bytes []byte) (result genericobjectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseAttachment(marshalUtil)
	return
}

// ParseAttachment is a wrapper for simplified unmarshaling of Attachments from a byte stream using the marshalUtil
// package.
func ParseAttachment(marshalUtil *marshalutil.MarshalUtil) (result *Attachment, err error) {
	result = &Attachment{}
	if result.transactionID, err = ledgerstate.TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse transaction ID in attachment: %w", err)
		return
	}
	if result.messageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse message ID in attachment: %w", err)
		return
	}

	return
}

// TransactionID returns the transactionID of this Attachment.
func (a *Attachment) TransactionID() ledgerstate.TransactionID {
	return a.transactionID
}

// MessageID returns the messageID of this Attachment.
func (a *Attachment) MessageID() MessageID {
	return a.messageID
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

// ObjectStorageKey returns the key that is used to store the object in the database.
func (a *Attachment) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(a.transactionID.Bytes(), a.MessageID().Bytes())
}

// ObjectStorageValue marshals the "content part" of an Attachment to a sequence of bytes. Since all of the information
// for this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (a *Attachment) ObjectStorageValue() (data []byte) {
	return
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ genericobjectstorage.StorableObject = &Attachment{}

// AttachmentLength holds the length of a marshaled Attachment in bytes.
const AttachmentLength = ledgerstate.TransactionIDLength + MessageIDLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MissingMessage ///////////////////////////////////////////////////////////////////////////////////////////////

// MissingMessage represents a missing message.
type MissingMessage struct {
	genericobjectstorage.StorableObjectFlags

	messageID    MessageID
	missingSince time.Time
}

// NewMissingMessage creates new missing message with the specified messageID.
func NewMissingMessage(messageID MessageID) *MissingMessage {
	return &MissingMessage{
		messageID:    messageID,
		missingSince: time.Now(),
	}
}

// FromObjectStorage creates an MissingMessage from sequences of key and bytes.
func (m *MissingMessage) FromObjectStorage(key, bytes []byte) (genericobjectstorage.StorableObject, error) {
	return m.FromBytes(byteutils.ConcatBytes(key, bytes))
}

// FromBytes parses the given bytes into a MissingMessage.
func (*MissingMessage) FromBytes(bytes []byte) (result genericobjectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MissingMessageFromMarshalUtil(marshalUtil)

	return
}

// MissingMessageFromMarshalUtil parses a MissingMessage from the given MarshalUtil.
func MissingMessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *MissingMessage, err error) {
	result = &MissingMessage{}

	if result.messageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse message ID of missing message: %w", err)
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse missingSince of missing message: %w", err)
		return
	}

	return
}

// MessageID returns the id of the message.
func (m *MissingMessage) MessageID() MessageID {
	return m.messageID
}

// MissingSince returns the time since when this message is missing.
func (m *MissingMessage) MissingSince() time.Time {
	return m.missingSince
}

// Bytes returns a marshaled version of this MissingMessage.
func (m *MissingMessage) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// ObjectStorageKey returns the key of the stored missing message.
// This returns the bytes of the messageID of the missing message.
func (m *MissingMessage) ObjectStorageKey() []byte {
	return m.messageID[:]
}

// ObjectStorageValue returns the value of the stored missing message.
func (m *MissingMessage) ObjectStorageValue() (result []byte) {
	result, err := m.missingSince.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ genericobjectstorage.StorableObject = &MissingMessage{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
