package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
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

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

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
		messageStorage:                    objectstorage.NewStructStorage[Message](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMessage), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		messageMetadataStorage:            objectstorage.NewStructStorage[MessageMetadata](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMessageMetadata), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:                   objectstorage.NewStructStorage[Approver](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixApprovers), cacheProvider.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, ApproverTypeLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		missingMessageStorage:             objectstorage.NewStructStorage[MissingMessage](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMissingMessage), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		attachmentStorage:                 objectstorage.NewStructStorage[Attachment](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixAttachments), cacheProvider.CacheTime(cacheTime), objectstorage.PartitionKey(Attachment{}.KeyPartitions()...), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		markerIndexBranchIDMappingStorage: objectstorage.NewStructStorage[MarkerIndexBranchIDMapping](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMarkerBranchIDMapping), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		branchVotersStorage:               objectstorage.NewStructStorage[BranchVoters](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixBranchVoters), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		latestBranchVotesStorage:          objectstorage.NewStructStorage[LatestBranchVotes](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixLatestBranchVotes), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		latestMarkerVotesStorage:          objectstorage.NewStructStorage[LatestMarkerVotes](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixLatestMarkerVotes), cacheProvider.CacheTime(approvalWeightCacheTime), LatestMarkerVotesKeyPartition, objectstorage.LeakDetectionEnabled(false)),
		branchWeightStorage:               objectstorage.NewStructStorage[BranchWeight](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixBranchWeight), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		markerMessageMappingStorage:       objectstorage.NewStructStorage[MarkerMessageMapping](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMarkerMessageMapping), cacheProvider.CacheTime(cacheTime), MarkerMessageMappingPartitionKeys, objectstorage.StoreOnCreation(true)),

		Events: newStorageEvents(),
	}

	storage.storeGenesis()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (s *Storage) Setup() {
	s.tangle.Parser.Events.MessageParsed.Hook(event.NewClosure(func(event *MessageParsedEvent) {
		s.tangle.Storage.StoreMessage(event.Message)
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
		s.tangle.Storage.Events.MissingMessageStored.Trigger(&MissingMessageStoredEvent{messageID})
	}

	// messages are stored, trigger MessageStored event to move on next check
	s.Events.MessageStored.Trigger(&MessageStoredEvent{message})
}

// Message retrieves a message from the message store.
func (s *Storage) Message(messageID MessageID) *objectstorage.CachedObject[*Message] {
	return s.messageStorage.Load(messageID.Bytes())
}

// MessageMetadata retrieves the MessageMetadata with the given MessageID.
func (s *Storage) MessageMetadata(messageID MessageID, computeIfAbsentCallback ...func() *MessageMetadata) *objectstorage.CachedObject[*MessageMetadata] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) *MessageMetadata {
			return computeIfAbsentCallback[0]()
		})
	}

	return s.messageMetadataStorage.Load(messageID.Bytes())
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
func (s *Storage) StoreAttachment(transactionID utxo.TransactionID, messageID MessageID) (cachedAttachment *objectstorage.CachedObject[*Attachment], stored bool) {
	return s.attachmentStorage.StoreIfAbsent(NewAttachment(transactionID, messageID))
}

// Attachments retrieves the attachment of a transaction in attachmentStorage.
func (s *Storage) Attachments(transactionID utxo.TransactionID) (cachedAttachments objectstorage.CachedObjects[*Attachment]) {
	s.attachmentStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Attachment]) bool {
		cachedAttachments = append(cachedAttachments, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(transactionID.Bytes()))
	return
}

// AttachmentMessageIDs returns the messageIDs of the transaction in attachmentStorage.
func (s *Storage) AttachmentMessageIDs(transactionID utxo.TransactionID) (messageIDs MessageIDs) {
	messageIDs = NewMessageIDs()
	s.Attachments(transactionID).Consume(func(attachment *Attachment) {
		messageIDs.Add(attachment.MessageID())
	})
	return
}

// IsTransactionAttachedByMessage checks whether Transaction with transactionID is attached by Message with messageID.
func (s *Storage) IsTransactionAttachedByMessage(transactionID utxo.TransactionID, messageID MessageID) (attached bool) {
	return s.attachmentStorage.Contains(NewAttachment(transactionID, messageID).ObjectStorageKey())
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (s *Storage) DeleteMessage(messageID MessageID) {
	s.Message(messageID).Consume(func(currentMsg *Message) {
		currentMsg.ForEachParent(func(parent Parent) {
			s.deleteApprover(parent, messageID)
		})

		s.messageMetadataStorage.Delete(messageID.Bytes())
		s.messageStorage.Delete(messageID.Bytes())

		s.Events.MessageRemoved.Trigger(&MessageRemovedEvent{messageID})
	})
}

// DeleteMissingMessage deletes a message from the missingMessageStorage.
func (s *Storage) DeleteMissingMessage(messageID MessageID) {
	s.missingMessageStorage.Delete(messageID.Bytes())
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
func (s *Storage) DeleteMarkerMessageMapping(branchID utxo.TransactionID, messageID MessageID) {
	s.markerMessageMappingStorage.Delete(byteutils.ConcatBytes(branchID.Bytes(), messageID.Bytes()))
}

// MarkerMessageMapping retrieves the MarkerMessageMapping associated with the given details.
func (s *Storage) MarkerMessageMapping(marker markers.Marker) (cachedMarkerMessageMappings *objectstorage.CachedObject[*MarkerMessageMapping]) {
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

// BranchVoters retrieves the BranchVoters with the given ledger.BranchID.
func (s *Storage) BranchVoters(branchID utxo.TransactionID, computeIfAbsentCallback ...func(branchID utxo.TransactionID) *BranchVoters) *objectstorage.CachedObject[*BranchVoters] {
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
func (s *Storage) BranchWeight(branchID utxo.TransactionID, computeIfAbsentCallback ...func(branchID utxo.TransactionID) *BranchWeight) *objectstorage.CachedObject[*BranchWeight] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.branchWeightStorage.ComputeIfAbsent(branchID.Bytes(), func(key []byte) *BranchWeight {
			return computeIfAbsentCallback[0](branchID)
		})
	}

	return s.branchWeightStorage.Load(branchID.Bytes())
}

func (s *Storage) storeGenesis() {
	s.MessageMetadata(EmptyMessageID, func() *MessageMetadata {
		genesisMetadata := model.NewStorable[MessageID, MessageMetadata](&messageMetadataModel{
			AddedBranchIDs:      utxo.NewTransactionIDs(),
			SubtractedBranchIDs: utxo.NewTransactionIDs(),
			SolidificationTime:  clock.SyncedTime().Add(time.Duration(-20) * time.Minute),
			Solid:               true,
			StructureDetails:    markers.NewStructureDetails(),
			Scheduled:           true,
			Booked:              true,
		})
		genesisMetadata.SetID(EmptyMessageID)
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
				res.SumSchedulerBookedTime += msgMetaData.ScheduledTime().Sub(msgMetaData.BookedTime())
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
				cachedApprovers := s.Approvers(messageMetadata.ID())
				if len(cachedApprovers) == 0 {
					tips = append(tips, messageMetadata.ID())
				}
				cachedApprovers.Release()
			}
		})
		return true
	})
	return tips
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
	model.StorableReference[approverSourceModel, MessageID] `serix:"0"`
}

type approverSourceModel struct {
	// the message which got referenced by the approver message.
	ReferencedMessageID MessageID `serix:"0"`

	// ApproverType defines if the reference was created by a strong, weak, shallowlike or shallowdislike parent reference.
	ApproverType ApproverType `serix:"1"`
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(approverType ApproverType, referencedMessageID MessageID, approverMessageID MessageID) *Approver {
	return &Approver{
		model.NewStorableReference[approverSourceModel, MessageID](approverSourceModel{
			ReferencedMessageID: referencedMessageID,
			ApproverType:        approverType,
		}, approverMessageID),
	}
}

// Type returns the type of the Approver reference.
func (a *Approver) Type() ApproverType {
	return a.SourceID.ApproverType
}

// ReferencedMessageID returns the ID of the message which is referenced by the approver.
func (a *Approver) ReferencedMessageID() MessageID {
	return a.SourceID.ReferencedMessageID
}

// ApproverMessageID returns the ID of the message which referenced the given approved message.
func (a *Approver) ApproverMessageID() MessageID {
	return a.TargetID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Attachment ///////////////////////////////////////////////////////////////////////////////////////////////////

// Attachment stores the information which transaction was attached by which message. We need this to be able to perform
// reverse lookups from transactions to their corresponding messages that attach them.
type Attachment struct {
	model.StorableReference[utxo.TransactionID, MessageID] `serix:"0"`
}

// NewAttachment creates an attachment object with the given information.
func NewAttachment(transactionID utxo.TransactionID, messageID MessageID) *Attachment {
	return &Attachment{model.NewStorableReference(transactionID, messageID)}
}

// TransactionID returns the transactionID of this Attachment.
func (a *Attachment) TransactionID() utxo.TransactionID {
	return a.SourceID
}

// MessageID returns the messageID of this Attachment.
func (a *Attachment) MessageID() MessageID {
	return a.TargetID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MissingMessage ///////////////////////////////////////////////////////////////////////////////////////////////

// MissingMessage represents a missing message.
type MissingMessage struct {
	model.Storable[MessageID, MissingMessage, *MissingMessage, time.Time] `serix:"0"`
}

// NewMissingMessage creates new missing message with the specified messageID.
func NewMissingMessage(messageID MessageID) *MissingMessage {
	now := time.Now()
	missingMessage := model.NewStorable[MessageID, MissingMessage](
		&now,
	)

	missingMessage.SetID(messageID)
	return missingMessage
}

// MessageID returns the id of the message.
func (m *MissingMessage) MessageID() MessageID {
	return m.ID()
}

// MissingSince returns the time since when this message is missing.
func (m *MissingMessage) MissingSince() time.Time {
	m.RLock()
	defer m.RUnlock()
	return m.M
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
