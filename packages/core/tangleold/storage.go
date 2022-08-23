package tangleold

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/node/clock"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

const (
	// PrefixBlock defines the storage prefix for block.
	PrefixBlock byte = iota

	// PrefixBlockMetadata defines the storage prefix for block metadata.
	PrefixBlockMetadata

	// PrefixChildren defines the storage prefix for children.
	PrefixChildren

	// PrefixMissingBlock defines the storage prefix for missing block.
	PrefixMissingBlock

	// PrefixAttachments defines the storage prefix for attachments.
	PrefixAttachments

	// PrefixMarkerConflictIDMapping defines the storage prefix for the PrefixMarkerConflictIDMapping.
	PrefixMarkerConflictIDMapping

	// PrefixConflictVoters defines the storage prefix for the ConflictVoters.
	PrefixConflictVoters

	// PrefixLatestConflictVotes defines the storage prefix for the LatestConflictVotes.
	PrefixLatestConflictVotes

	// PrefixLatestMarkerVotes defines the storage prefix for the LatestMarkerVotes.
	PrefixLatestMarkerVotes

	// PrefixConflictWeight defines the storage prefix for the ConflictWeight.
	PrefixConflictWeight

	// PrefixMarkerBlockMapping defines the storage prefix for the MarkerBlockMapping.
	PrefixMarkerBlockMapping

	// DBSequenceNumber defines the db sequence number.
	DBSequenceNumber = "seq"

	// cacheTime defines the number of seconds an object will wait in storage cache.
	cacheTime = 2 * time.Second

	// approvalWeightCacheTime defines the number of seconds an object related to approval weight will wait in storage cache.
	approvalWeightCacheTime = 20 * time.Second
)

// region storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage represents the storage of blocks.
type Storage struct {
	tangle                              *Tangle
	blockStorage                        *objectstorage.ObjectStorage[*Block]
	blockMetadataStorage                *objectstorage.ObjectStorage[*BlockMetadata]
	childStorage                        *objectstorage.ObjectStorage[*Child]
	missingBlockStorage                 *objectstorage.ObjectStorage[*MissingBlock]
	attachmentStorage                   *objectstorage.ObjectStorage[*Attachment]
	markerIndexConflictIDMappingStorage *objectstorage.ObjectStorage[*MarkerIndexConflictIDMapping]
	conflictVotersStorage               *objectstorage.ObjectStorage[*ConflictVoters]
	latestConflictVotesStorage          *objectstorage.ObjectStorage[*LatestConflictVotes]
	latestMarkerVotesStorage            *objectstorage.ObjectStorage[*LatestMarkerVotes]
	conflictWeightStorage               *objectstorage.ObjectStorage[*ConflictWeight]
	markerBlockMappingStorage           *objectstorage.ObjectStorage[*MarkerBlockMapping]

	Events   *StorageEvents
	shutdown chan struct{}
}

// NewStorage creates a new Storage.
func NewStorage(tangle *Tangle) (storage *Storage) {
	cacheProvider := tangle.Options.CacheTimeProvider

	storage = &Storage{
		tangle:                              tangle,
		shutdown:                            make(chan struct{}),
		blockStorage:                        objectstorage.NewStructStorage[Block](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixBlock), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		blockMetadataStorage:                objectstorage.NewStructStorage[BlockMetadata](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixBlockMetadata), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		childStorage:                        objectstorage.NewStructStorage[Child](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixChildren), cacheProvider.CacheTime(cacheTime), objectstorage.PartitionKey(BlockIDLength, ChildTypeLength, BlockIDLength), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		missingBlockStorage:                 objectstorage.NewStructStorage[MissingBlock](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMissingBlock), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		attachmentStorage:                   objectstorage.NewStructStorage[Attachment](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixAttachments), cacheProvider.CacheTime(cacheTime), objectstorage.PartitionKey(new(Attachment).KeyPartitions()...), objectstorage.LeakDetectionEnabled(false), objectstorage.StoreOnCreation(true)),
		markerIndexConflictIDMappingStorage: objectstorage.NewStructStorage[MarkerIndexConflictIDMapping](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMarkerConflictIDMapping), cacheProvider.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		conflictVotersStorage:               objectstorage.NewStructStorage[ConflictVoters](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixConflictVoters), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		latestConflictVotesStorage:          objectstorage.NewStructStorage[LatestConflictVotes](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixLatestConflictVotes), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		latestMarkerVotesStorage:            objectstorage.NewStructStorage[LatestMarkerVotes](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixLatestMarkerVotes), cacheProvider.CacheTime(approvalWeightCacheTime), LatestMarkerVotesKeyPartition, objectstorage.LeakDetectionEnabled(false)),
		conflictWeightStorage:               objectstorage.NewStructStorage[ConflictWeight](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixConflictWeight), cacheProvider.CacheTime(approvalWeightCacheTime), objectstorage.LeakDetectionEnabled(false)),
		markerBlockMappingStorage:           objectstorage.NewStructStorage[MarkerBlockMapping](objectstorage.NewStoreWithRealm(tangle.Options.Store, database.PrefixTangle, PrefixMarkerBlockMapping), cacheProvider.CacheTime(cacheTime), MarkerBlockMappingPartitionKeys, objectstorage.StoreOnCreation(true)),

		Events: newStorageEvents(),
	}

	storage.storeGenesis()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (s *Storage) Setup() {
	s.tangle.Parser.Events.BlockParsed.Hook(event.NewClosure(func(event *BlockParsedEvent) {
		s.tangle.Storage.StoreBlock(event.Block)
	}))
}

// StoreBlock stores a new block to the block store.
func (s *Storage) StoreBlock(block *Block) {
	// retrieve BlockID
	blockID := block.ID()

	// store Blocks only once by using the existence of the Metadata as a guard
	storedMetadata, stored := s.blockMetadataStorage.StoreIfAbsent(NewBlockMetadata(blockID))
	if !stored {
		return
	}

	// create typed version of the stored BlockMetadata
	cachedBlkMetadata := storedMetadata
	defer cachedBlkMetadata.Release()

	// store Block
	cachedBlock := s.blockStorage.Store(block)
	defer cachedBlock.Release()

	block.ForEachParent(func(parent Parent) {
		s.childStorage.Store(NewChild(ParentTypeToChildType[parent.Type], parent.ID, blockID)).Release()
	})

	// trigger events
	if s.missingBlockStorage.DeleteIfPresent(blockID.Bytes()) {
		s.tangle.Storage.Events.MissingBlockStored.Trigger(&MissingBlockStoredEvent{blockID})
	}

	// blocks are stored, trigger BlockStored event to move on next check
	s.Events.BlockStored.Trigger(&BlockStoredEvent{block})
}

// Block retrieves a block from the block store.
func (s *Storage) Block(blockID BlockID) *objectstorage.CachedObject[*Block] {
	return s.blockStorage.Load(blockID.Bytes())
}

// BlockMetadata retrieves the BlockMetadata with the given BlockID.
func (s *Storage) BlockMetadata(blockID BlockID, computeIfAbsentCallback ...func() *BlockMetadata) *objectstorage.CachedObject[*BlockMetadata] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.blockMetadataStorage.ComputeIfAbsent(blockID.Bytes(), func(key []byte) *BlockMetadata {
			return computeIfAbsentCallback[0]()
		})
	}

	return s.blockMetadataStorage.Load(blockID.Bytes())
}

// Children retrieves the Children of a Block from the object storage. It is possible to provide an optional
// ChildType to only return the corresponding Children.
func (s *Storage) Children(blockID BlockID, optionalChildType ...ChildType) (cachedChildren objectstorage.CachedObjects[*Child]) {
	var iterationPrefix []byte
	if len(optionalChildType) >= 1 {
		iterationPrefix = byteutils.ConcatBytes(blockID.Bytes(), optionalChildType[0].Bytes())
	} else {
		iterationPrefix = blockID.Bytes()
	}

	cachedChildren = make(objectstorage.CachedObjects[*Child], 0)
	s.childStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Child]) bool {
		cachedChildren = append(cachedChildren, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(iterationPrefix))

	return
}

// StoreMissingBlock stores a new MissingBlock entry in the object storage.
func (s *Storage) StoreMissingBlock(missingBlock *MissingBlock) (cachedMissingBlock *objectstorage.CachedObject[*MissingBlock], stored bool) {
	cachedObject, stored := s.missingBlockStorage.StoreIfAbsent(missingBlock)
	cachedMissingBlock = cachedObject

	return
}

// MissingBlocks return the ids of blocks in missingBlockStorage
func (s *Storage) MissingBlocks() (ids []BlockID) {
	s.missingBlockStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MissingBlock]) bool {
		cachedObject.Consume(func(object *MissingBlock) {
			ids = append(ids, object.BlockID())
		})

		return true
	})
	return
}

// StoreAttachment stores a new attachment if not already stored.
func (s *Storage) StoreAttachment(transactionID utxo.TransactionID, blockID BlockID) (cachedAttachment *objectstorage.CachedObject[*Attachment], stored bool) {
	return s.attachmentStorage.StoreIfAbsent(NewAttachment(transactionID, blockID))
}

// Attachments retrieves the attachment of a transaction in attachmentStorage.
func (s *Storage) Attachments(transactionID utxo.TransactionID) (cachedAttachments objectstorage.CachedObjects[*Attachment]) {
	s.attachmentStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Attachment]) bool {
		cachedAttachments = append(cachedAttachments, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(transactionID.Bytes()))
	return
}

// AttachmentBlockIDs returns the blockIDs of the transaction in attachmentStorage.
func (s *Storage) AttachmentBlockIDs(transactionID utxo.TransactionID) (blockIDs BlockIDs) {
	blockIDs = NewBlockIDs()
	s.Attachments(transactionID).Consume(func(attachment *Attachment) {
		blockIDs.Add(attachment.BlockID())
	})
	return
}

// IsTransactionAttachedByBlock checks whether Transaction with transactionID is attached by Block with blockID.
func (s *Storage) IsTransactionAttachedByBlock(transactionID utxo.TransactionID, blockID BlockID) (attached bool) {
	return s.attachmentStorage.Contains(NewAttachment(transactionID, blockID).ObjectStorageKey())
}

// DeleteBlock deletes a block and its association to parents by un-marking the given
// block as an child.
func (s *Storage) DeleteBlock(blockID BlockID) {
	s.Block(blockID).Consume(func(currentBlk *Block) {
		currentBlk.ForEachParent(func(parent Parent) {
			s.deleteChild(parent, blockID)
		})

		s.blockMetadataStorage.Delete(blockID.Bytes())
		s.blockStorage.Delete(blockID.Bytes())

		s.Events.BlockRemoved.Trigger(&BlockRemovedEvent{blockID})
	})
}

// DeleteMissingBlock deletes a block from the missingBlockStorage.
func (s *Storage) DeleteMissingBlock(blockID BlockID) {
	s.missingBlockStorage.Delete(blockID.Bytes())
}

// MarkerIndexConflictIDMapping retrieves the MarkerIndexConflictIDMapping for the given SequenceID. It accepts an optional
// computeIfAbsent callback that can be used to dynamically create a MarkerIndexConflictIDMapping if it doesn't exist,
// yet.
func (s *Storage) MarkerIndexConflictIDMapping(sequenceID markers.SequenceID, computeIfAbsentCallback ...func(sequenceID markers.SequenceID) *MarkerIndexConflictIDMapping) *objectstorage.CachedObject[*MarkerIndexConflictIDMapping] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.markerIndexConflictIDMappingStorage.ComputeIfAbsent(sequenceID.Bytes(), func(key []byte) *MarkerIndexConflictIDMapping {
			return computeIfAbsentCallback[0](sequenceID)
		})
	}

	return s.markerIndexConflictIDMappingStorage.Load(sequenceID.Bytes())
}

// StoreMarkerBlockMapping stores a MarkerBlockMapping in the underlying object storage.
func (s *Storage) StoreMarkerBlockMapping(markerBlockMapping *MarkerBlockMapping) {
	s.markerBlockMappingStorage.Store(markerBlockMapping).Release()
}

// DeleteMarkerBlockMapping deleted a MarkerBlockMapping in the underlying object storage.
func (s *Storage) DeleteMarkerBlockMapping(conflictID utxo.TransactionID, blockID BlockID) {
	s.markerBlockMappingStorage.Delete(byteutils.ConcatBytes(conflictID.Bytes(), blockID.Bytes()))
}

// MarkerBlockMapping retrieves the MarkerBlockMapping associated with the given details.
func (s *Storage) MarkerBlockMapping(marker markers.Marker) (cachedMarkerBlockMappings *objectstorage.CachedObject[*MarkerBlockMapping]) {
	return s.markerBlockMappingStorage.Load(marker.Bytes())
}

// MarkerBlockMappings retrieves the MarkerBlockMappings of a Sequence in the object storage.
func (s *Storage) MarkerBlockMappings(sequenceID markers.SequenceID) (cachedMarkerBlockMappings objectstorage.CachedObjects[*MarkerBlockMapping]) {
	s.markerBlockMappingStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MarkerBlockMapping]) bool {
		cachedMarkerBlockMappings = append(cachedMarkerBlockMappings, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(sequenceID.Bytes()))
	return
}

// ConflictVoters retrieves the ConflictVoters with the given ledger.ConflictID.
func (s *Storage) ConflictVoters(conflictID utxo.TransactionID, computeIfAbsentCallback ...func(conflictID utxo.TransactionID) *ConflictVoters) *objectstorage.CachedObject[*ConflictVoters] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictVotersStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) *ConflictVoters {
			return computeIfAbsentCallback[0](conflictID)
		})
	}

	return s.conflictVotersStorage.Load(conflictID.Bytes())
}

// LatestConflictVotes retrieves the LatestConflictVotes of the given Voter.
func (s *Storage) LatestConflictVotes(voter Voter, computeIfAbsentCallback ...func(voter Voter) *LatestConflictVotes) *objectstorage.CachedObject[*LatestConflictVotes] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.latestConflictVotesStorage.ComputeIfAbsent(byteutils.ConcatBytes(voter.Bytes()), func(key []byte) *LatestConflictVotes {
			return computeIfAbsentCallback[0](voter)
		})
	}

	return s.latestConflictVotesStorage.Load(byteutils.ConcatBytes(voter.Bytes()))
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

// ConflictWeight retrieves the ConflictWeight with the given ConflictID.
func (s *Storage) ConflictWeight(conflictID utxo.TransactionID, computeIfAbsentCallback ...func(conflictID utxo.TransactionID) *ConflictWeight) *objectstorage.CachedObject[*ConflictWeight] {
	if len(computeIfAbsentCallback) >= 1 {
		return s.conflictWeightStorage.ComputeIfAbsent(conflictID.Bytes(), func(key []byte) *ConflictWeight {
			return computeIfAbsentCallback[0](conflictID)
		})
	}

	return s.conflictWeightStorage.Load(conflictID.Bytes())
}

func (s *Storage) storeGenesis() {
	s.BlockMetadata(EmptyBlockID, func() *BlockMetadata {
		genesisMetadata := model.NewStorable[BlockID, BlockMetadata](&blockMetadataModel{
			AddedConflictIDs:      utxo.NewTransactionIDs(),
			SubtractedConflictIDs: utxo.NewTransactionIDs(),
			SolidificationTime:    clock.SyncedTime().Add(time.Duration(-20) * time.Minute),
			Solid:                 true,
			StructureDetails:      markers.NewStructureDetails(),
			Scheduled:             true,
			Booked:                true,
		})
		genesisMetadata.SetID(EmptyBlockID)
		return genesisMetadata
	}).Release()
}

// deleteChild deletes the Child from the object storage that was created by the specified parent.
func (s *Storage) deleteChild(parent Parent, approvingBlock BlockID) {
	s.childStorage.Delete(byteutils.ConcatBytes(parent.ID.Bytes(), ParentTypeToChildType[parent.Type].Bytes(), approvingBlock.Bytes()))
}

// Shutdown marks the tangle as stopped, so it will not accept any new blocks (waits for all backgroundTasks to finish).
func (s *Storage) Shutdown() {
	s.blockStorage.Shutdown()
	fmt.Println("======= Shutdown block metadate starts")
	s.blockMetadataStorage.Shutdown()
	fmt.Println("======= Shutdown block metadate ends")
	s.childStorage.Shutdown()
	s.missingBlockStorage.Shutdown()
	s.attachmentStorage.Shutdown()
	s.markerIndexConflictIDMappingStorage.Shutdown()
	s.conflictVotersStorage.Shutdown()
	s.latestConflictVotesStorage.Shutdown()
	s.latestMarkerVotesStorage.Shutdown()
	s.conflictWeightStorage.Shutdown()
	s.markerBlockMappingStorage.Shutdown()

	close(s.shutdown)
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
func (s *Storage) Prune() error {
	for _, storagePrune := range []func() error{
		s.blockStorage.Prune,
		s.blockMetadataStorage.Prune,
		s.childStorage.Prune,
		s.missingBlockStorage.Prune,
		s.attachmentStorage.Prune,
		s.markerIndexConflictIDMappingStorage.Prune,
		s.conflictVotersStorage.Prune,
		s.latestConflictVotesStorage.Prune,
		s.latestMarkerVotesStorage.Prune,
		s.conflictWeightStorage.Prune,
		s.markerBlockMappingStorage.Prune,
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
	MissingBlockCount             int
}

// DBStats returns the number of solid blocks and total number of blocks in the database (blockMetadataStorage,
// that should contain the blocks as blockStorage), the number of blocks in missingBlockStorage, furthermore
// the average time it takes to solidify blocks.
func (s *Storage) DBStats() (res DBStatsResult) {
	s.blockMetadataStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*BlockMetadata]) bool {
		cachedObject.Consume(func(blkMetaData *BlockMetadata) {
			res.StoredCount++
			received := blkMetaData.ReceivedTime()
			if blkMetaData.IsSolid() {
				res.SolidCount++
				res.SumSolidificationReceivedTime += blkMetaData.SolidificationTime().Sub(received)
			}
			if blkMetaData.IsBooked() {
				res.BookedCount++
				res.SumBookedReceivedTime += blkMetaData.BookedTime().Sub(received)
			}
			if blkMetaData.Scheduled() {
				res.ScheduledCount++
				res.SumSchedulerReceivedTime += blkMetaData.ScheduledTime().Sub(received)
				res.SumSchedulerBookedTime += blkMetaData.ScheduledTime().Sub(blkMetaData.BookedTime())
			}
		})
		return true
	})

	s.missingBlockStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*MissingBlock]) bool {
		cachedObject.Consume(func(object *MissingBlock) {
			res.MissingBlockCount++
		})
		return true
	})
	return
}

// RetrieveAllTips returns the tips (i.e., solid blocks that are not part of the children list).
// It iterates over the blockMetadataStorage, thus only use this method if necessary.
// TODO: improve this function.
func (s *Storage) RetrieveAllTips() (tips []BlockID) {
	s.blockMetadataStorage.ForEach(func(key []byte, cachedBlock *objectstorage.CachedObject[*BlockMetadata]) bool {
		cachedBlock.Consume(func(blockMetadata *BlockMetadata) {
			if blockMetadata != nil && blockMetadata.IsSolid() {
				cachedChildren := s.Children(blockMetadata.ID())
				if len(cachedChildren) == 0 {
					tips = append(tips, blockMetadata.ID())
				}
				cachedChildren.Release()
			}
		})
		return true
	})
	return tips
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildType /////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// StrongChild is the ChildType that represents references formed by strong parents.
	StrongChild ChildType = iota

	// WeakChild is the ChildType that represents references formed by weak parents.
	WeakChild

	// ShallowLikeChild is the ChildType that represents references formed by shallow like parents.
	ShallowLikeChild
)

// ChildTypeLength contains the amount of bytes that a marshaled version of the ChildType contains.
const ChildTypeLength = 1

// ChildType is a type that represents the different kind of reverse mapping that we have for references formed by
// strong and weak parents.
type ChildType uint8

// ParentTypeToChildType represents a convenient mapping between a parent type and the child type.
var ParentTypeToChildType = map[ParentsType]ChildType{
	StrongParentType:      StrongChild,
	WeakParentType:        WeakChild,
	ShallowLikeParentType: ShallowLikeChild,
}

// Bytes returns a marshaled version of the ChildType.
func (a ChildType) Bytes() []byte {
	return []byte{byte(a)}
}

// String returns a human readable version of the ChildType.
func (a ChildType) String() string {
	switch a {
	case StrongChild:
		return "ChildType(StrongChild)"
	case WeakChild:
		return "ChildType(WeakChild)"
	case ShallowLikeChild:
		return "ChildType(ShallowLikeChild)"
	default:
		return fmt.Sprintf("ChildType(%X)", uint8(a))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Child /////////////////////////////////////////////////////////////////////////////////////////////////////

// Child is an child of a given referenced block.
type Child struct {
	model.StorableReference[Child, *Child, childSourceModel, BlockID] `serix:"0"`
}

type childSourceModel struct {
	// the block which got referenced by the child block.
	ReferencedBlockID BlockID `serix:"0"`

	// ChildType defines if the reference was created by a strong, weak, shallowlike or shallowdislike parent reference.
	ChildType ChildType `serix:"1"`
}

// NewChild creates a new child relation to the given approved/referenced block.
func NewChild(childType ChildType, referencedBlockID BlockID, childBlockID BlockID) *Child {
	return model.NewStorableReference[Child](childSourceModel{
		ReferencedBlockID: referencedBlockID,
		ChildType:         childType,
	}, childBlockID)
}

// Type returns the type of the Child reference.
func (a *Child) Type() ChildType {
	return a.SourceID().ChildType
}

// ReferencedBlockID returns the ID of the block which is referenced by the child.
func (a *Child) ReferencedBlockID() BlockID {
	return a.SourceID().ReferencedBlockID
}

// ChildBlockID returns the ID of the block which referenced the given approved block.
func (a *Child) ChildBlockID() BlockID {
	return a.TargetID()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Attachment ///////////////////////////////////////////////////////////////////////////////////////////////////

// Attachment stores the information which transaction was attached by which block. We need this to be able to perform
// reverse lookups from transactions to their corresponding blocks that attach them.
type Attachment struct {
	model.StorableReference[Attachment, *Attachment, utxo.TransactionID, BlockID] `serix:"0"`
}

// NewAttachment creates an attachment object with the given information.
func NewAttachment(transactionID utxo.TransactionID, blockID BlockID) *Attachment {
	return model.NewStorableReference[Attachment](transactionID, blockID)
}

// TransactionID returns the transactionID of this Attachment.
func (a *Attachment) TransactionID() utxo.TransactionID {
	return a.SourceID()
}

// BlockID returns the blockID of this Attachment.
func (a *Attachment) BlockID() BlockID {
	return a.TargetID()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MissingBlock ///////////////////////////////////////////////////////////////////////////////////////////////

// MissingBlock represents a missing block.
type MissingBlock struct {
	model.Storable[BlockID, MissingBlock, *MissingBlock, time.Time] `serix:"0"`
}

// NewMissingBlock creates new missing block with the specified blockID.
func NewMissingBlock(blockID BlockID) *MissingBlock {
	now := time.Now()
	missingBlock := model.NewStorable[BlockID, MissingBlock](
		&now,
	)

	missingBlock.SetID(blockID)
	return missingBlock
}

// BlockID returns the id of the block.
func (m *MissingBlock) BlockID() BlockID {
	return m.ID()
}

// MissingSince returns the time since when this block is missing.
func (m *MissingBlock) MissingSince() time.Time {
	m.RLock()
	defer m.RUnlock()
	return m.M
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
