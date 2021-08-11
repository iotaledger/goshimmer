package tangle

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	// MessageVersion defines the version of the message structure.
	MessageVersion uint8 = 1

	// MaxMessageSize defines the maximum size of a message.
	MaxMessageSize = 64 * 1024

	// MessageIDLength defines the length of an MessageID.
	MessageIDLength = 32

	// MinParentsCount defines the minimum number of parents each parents block must have.
	MinParentsCount = 1

	// MaxParentsCount defines the maximum number of parents each parents block must have.
	MaxParentsCount = 8

	// MinStrongParentsCount defines the minimum number of strong parents a message must have.
	MinStrongParentsCount = 1
)

// region MessageID ////////////////////////////////////////////////////////////////////////////////////////////////////

// MessageID identifies a message via its BLAKE2b-256 hash of its bytes.
type MessageID [MessageIDLength]byte

// EmptyMessageID is an empty id.
var EmptyMessageID = MessageID{}

// NewMessageID creates a new message id.
func NewMessageID(base58EncodedString string) (result MessageID, err error) {
	msgIDBytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		err = fmt.Errorf("failed to decode base58 encoded string '%s': %w", base58EncodedString, err)

		return
	}

	if len(msgIDBytes) != MessageIDLength {
		err = fmt.Errorf("length of base58 formatted message id is wrong")

		return
	}

	copy(result[:], msgIDBytes)

	return
}

// MessageIDFromBytes unmarshals a message id from a sequence of bytes.
func MessageIDFromBytes(bytes []byte) (result MessageID, consumedBytes int, err error) {
	// check arguments
	if len(bytes) < MessageIDLength {
		err = fmt.Errorf("bytes not long enough to encode a valid message id")
		return
	}

	// calculate result
	copy(result[:], bytes)

	// return the number of bytes we processed
	consumedBytes = MessageIDLength

	return
}

// ReferenceFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ReferenceFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (MessageID, error) {
	id, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return MessageIDFromBytes(data) })
	if err != nil {
		err = fmt.Errorf("failed to parse message ID: %w", err)
		return MessageID{}, err
	}
	return id.(MessageID), nil
}

// MarshalBinary marshals the MessageID into bytes.
func (id *MessageID) MarshalBinary() (result []byte, err error) {
	return id.Bytes(), nil
}

// UnmarshalBinary unmarshals the bytes into an MessageID.
func (id *MessageID) UnmarshalBinary(data []byte) (err error) {
	if len(data) != MessageIDLength {
		err = fmt.Errorf("data must be exactly %d long to encode a valid message id", MessageIDLength)
		return
	}
	copy(id[:], data)

	return
}

// Bytes returns the bytes of the MessageID.
func (id MessageID) Bytes() []byte {
	return id[:]
}

// Base58 returns a base58 encoded version of the MessageID.
func (id MessageID) Base58() string {
	return base58.Encode(id[:])
}

// CompareTo does a lexicographical comparison to another messageID.
// Returns 0 if equal, -1 if smaller, or 1 if larger than other.
// Passing nil as other will result in a panic.
func (id MessageID) CompareTo(other MessageID) int {
	return bytes.Compare(id.Bytes(), other.Bytes())
}

// String returns a human readable representation of the MessageID.
func (id MessageID) String() string {
	if id == EmptyMessageID {
		return "MessageID(EmptyMessageID)"
	}

	if messageIDAlias, exists := messageIDAliases[id]; exists {
		return "MessageID(" + messageIDAlias + ")"
	}

	return "MessageID(" + base58.Encode(id[:]) + ")"
}

// messageIDAliases contains a list of aliases registered for a set of MessageIDs.
var messageIDAliases = make(map[MessageID]string)

// RegisterMessageIDAlias registers an alias that will modify the String() output of the MessageID to show a human
// readable string instead of the base58 encoded version of itself.
func RegisterMessageIDAlias(messageID MessageID, alias string) {
	messageIDAliases[messageID] = alias
}

// UnregisterMessageIDAliases removes all aliases registered through the RegisterMessageIDAlias function.
func UnregisterMessageIDAliases() {
	messageIDAliases = make(map[MessageID]string)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageIDs ///////////////////////////////////////////////////////////////////////////////////////////////////

// MessageIDs is a slice of MessageID.
type MessageIDs []MessageID

// ToStrings converts a slice of MessageIDs to a slice of strings.
func (ids MessageIDs) ToStrings() []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, id.Base58())
	}
	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Message //////////////////////////////////////////////////////////////////////////////////////////////////////

type ParentsType uint8

const (
	StrongParentType ParentsType = iota
	WeakParentType
	DislikeParentType
	LikeParentType

	// NumberOfBlockTypes counts StrongParents, WeakParents, DislikeParents, LikeParents.
	// it must be placed after the declaration of all block types.
	NumberOfBlockTypes

	// NumberOfUniqueBlocks counts the blocks that may not have their parents repeat in other blocks.
	// Currently it is only Weak and Dislike blocks.
	NumberOfUniqueBlocks = 2
)

// String returns string representation of ParentsType
func (bp ParentsType) String() string {
	return []string{"Strong Parent", "Weak Parent", "Dislike Parent", "Like Parent"}[bp]
}

type ParentsBlock struct {
	ParentsType
	ParentsCount uint8
	References   MessageIDs
}

// Message represents the core message for the base layer Tangle.
type Message struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (get sent over the wire)
	version            uint8
	parentsBlocksCount uint8
	parentsBlocks      []ParentsBlock
	issuerPublicKey    ed25519.PublicKey
	issuingTime        time.Time
	sequenceNumber     uint64
	payload            payload.Payload
	nonce              uint64
	signature          ed25519.Signature

	// derived properties
	id         *MessageID
	idMutex    sync.RWMutex
	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewMessage creates a new message with the details provided by the issuer.
func NewMessage(strongParents, weakParents, dislikeParents, likeParents MessageIDs, issuingTime time.Time,
	issuerPublicKey ed25519.PublicKey, sequenceNumber uint64, msgPayload payload.Payload, nonce uint64, signature ed25519.Signature) (*Message, error) {
	// remove duplicates, sort in ASC
	sortedStrongParents := sortParents(strongParents)
	sortedWeakParents := sortParents(weakParents)
	sortedDislikeParents := sortParents(dislikeParents)
	sortedLikeParents := sortParents(likeParents)

	strongParentsCount := len(sortedStrongParents)
	weakParentsCount := len(sortedWeakParents)
	dislikeParentsCount := len(sortedDislikeParents)
	likeParentsCount := len(sortedLikeParents)

	var parentsBlocks []ParentsBlock

	parentsBlocks = append(parentsBlocks, ParentsBlock{
		ParentsType:  StrongParentType,
		ParentsCount: uint8(strongParentsCount),
		References:   sortedStrongParents,
	})

	if weakParentsCount > 0 {
		parentsBlocks = append(parentsBlocks, ParentsBlock{
			ParentsType:  WeakParentType,
			ParentsCount: uint8(weakParentsCount),
			References:   sortedWeakParents,
		})
	}

	if dislikeParentsCount > 0 {
		parentsBlocks = append(parentsBlocks, ParentsBlock{
			ParentsType:  DislikeParentType,
			ParentsCount: uint8(dislikeParentsCount),
			References:   sortedDislikeParents,
		})
	}

	if likeParentsCount > 0 {
		parentsBlocks = append(parentsBlocks, ParentsBlock{
			ParentsType:  LikeParentType,
			ParentsCount: uint8(likeParentsCount),
			References:   sortedLikeParents,
		})
	}

	return newMessageWithValidation(MessageVersion, parentsBlocks, issuingTime, issuerPublicKey, msgPayload, nonce, signature, sequenceNumber)
}

/**
newMessageWithValidation creates a new message while performing ths following syntactical checks:
1. A Strong Parents Block must exist.
2. Parents Block types cannot repeat.
3. Parent count per block 1 <= x <= 8.
4. Parents unique within block.
5. Parents lexicographically sorted within block.
6. ParentsCount within each block must match the parents actually parsed within the block.
7. A Parent(s) repetition is only allowed when it occurs across Strong and Like parents
8. Blocks should be ordered by type in ascending order
**/
func newMessageWithValidation(version uint8, parentsBlocks []ParentsBlock, issuingTime time.Time,
	issuerPublicKey ed25519.PublicKey, msgPayload payload.Payload, nonce uint64,
	signature ed25519.Signature, sequenceNumber uint64) (result *Message, err error) {
	// Validate strong parent block
	if len(parentsBlocks) == 0 || parentsBlocks[0].ParentsType != StrongParentType ||
		len(parentsBlocks[StrongParentType].References) < MinStrongParentsCount {
		return nil, ErrNoStrongParents
	}

	// Block types must be ordered in ASC order and not repeat
	for i := 0; i < len(parentsBlocks)-1; i++ {
		if parentsBlocks[i].ParentsType == parentsBlocks[i+1].ParentsType {
			return nil, ErrRepeatingBlockTypes
		}
		if parentsBlocks[i].ParentsType > parentsBlocks[i+1].ParentsType {
			return nil, ErrBlocksNotOrderedByType
		}
		// we can skip the first block because we already ascertained it is of StrongParentType
		if parentsBlocks[i+1].ParentsType >= NumberOfBlockTypes {
			return nil, ErrBlockTypeIsUnknown
		}
	}

	// 1. Parent Count is correct for each block
	// 2. Number of parents in eac block is in range
	// 3. Parents are lexicographically ordered with no repetitions
	for _, block := range parentsBlocks {
		if int(block.ParentsCount) != len(block.References) {
			return nil, ErrParentsCountMismatch
		}
		if block.ParentsCount > MaxParentsCount || block.ParentsCount < MinParentsCount {
			return nil, ErrParentsOutOfRange
		}
		// The lexicographical order check also makes sure there are no duplicates
		for i := 0; i < len(block.References)-1; i++ {
			switch block.References[i].CompareTo(block.References[i+1]) {
			case 0:
				return nil, ErrRepeatingReferencesInBlock
			case 1:
				return nil, ErrParentsNotLexicographicallyOrdered
			}
		}
	}

	if !referencesUniqueAcrossBlocks(parentsBlocks) {
		return nil, ErrRepeatingMessagesAcrossBlocks
	}

	return &Message{
		version:            version,
		parentsBlocksCount: uint8(len(parentsBlocks)),
		parentsBlocks:      parentsBlocks,
		issuerPublicKey:    issuerPublicKey,
		issuingTime:        issuingTime,
		sequenceNumber:     sequenceNumber,
		payload:            msgPayload,
		nonce:              nonce,
		signature:          signature,
	}, nil
}

// validate messagesIDs are unique across blocks
// there may be repetition across strong and like parents
func referencesUniqueAcrossBlocks(parentsBlocks []ParentsBlock) bool {
	combinedMessageMap := make(map[MessageID]types.Empty, NumberOfBlockTypes*MaxParentsCount)
	parentsArray := make(MessageIDs, 0, MaxParentsCount*NumberOfUniqueBlocks)
	for _, block := range parentsBlocks {
		// combine strong parent and like parents
		if block.ParentsType == StrongParentType || block.ParentsType == LikeParentType {
			for _, parent := range block.References {
				combinedMessageMap[parent] = types.Void
			}
		} else {
			parentsArray = append(parentsArray, block.References...)
		}
	}
	expectedLength := len(combinedMessageMap) + len(parentsArray)
	for _, parent := range parentsArray {
		combinedMessageMap[parent] = types.Void
	}

	return expectedLength == len(combinedMessageMap)
}

// filters and sorts given parents and returns a new slice with sorted parents
func sortParents(parents MessageIDs) (sorted MessageIDs) {
	seen := make(map[MessageID]types.Empty)
	sorted = make(MessageIDs, 0, len(parents))

	// filter duplicates
	for _, parent := range parents {
		if _, seenAlready := seen[parent]; seenAlready {
			continue
		}
		seen[parent] = types.Void
		sorted = append(sorted, parent)
	}

	// sort parents
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].Bytes(), sorted[j].Bytes()) < 0
	})

	return
}

// MessageFromBytes parses the given bytes into a message.
func MessageFromBytes(bytes []byte) (result *Message, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MessageFromMarshalUtil(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	if len(bytes) != consumedBytes {
		err = errors.Errorf("consumed bytes %d not equal total bytes %d: %w", consumedBytes, len(bytes), cerrors.ErrParseBytesFailed)
	}
	return
}

// MessageFromMarshalUtil parses a message from the given marshal util.
func MessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*Message, error) {
	// determine read offset before starting to parse
	readOffsetStart := marshalUtil.ReadOffset()

	// parse information
	version, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, errors.Errorf("failed to parse message version from MarshalUtil: %w", err)
	}

	var parentsBlocksCount uint8
	if parentsBlocksCount, err = marshalUtil.ReadByte(); err != nil {
		return nil, errors.Errorf("failed to parse parents count from MarshalUtil: %w", err)
	}
	if parentsBlocksCount < MinParentsCount || parentsBlocksCount > MaxParentsCount {
		return nil, errors.Errorf("parents count %d not allowed: %w", parentsBlocksCount, cerrors.ErrParseBytesFailed)
	}

	parentsBlocks := make([]ParentsBlock, parentsBlocksCount)

	for i := 0; i < int(parentsBlocksCount); i++ {
		var parentType uint8
		if parentType, err = marshalUtil.ReadByte(); err != nil {
			return nil, errors.Errorf("failed to parse parent types from MarshalUtil: %w", err)
		}

		var parentsCount uint8
		if parentsCount, err = marshalUtil.ReadByte(); err != nil {
			return nil, errors.Errorf("failed to parse parents count from MarshalUtil: %w", err)
		}
		references := make(MessageIDs, parentsCount)
		for j := 0; j < int(parentsCount); j++ {
			if references[j], err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
				return nil, errors.Errorf("failed to parse parent %d-%d from MarshalUtil: %w", i, j, err)
			}
		}
		parentsBlocks[i] = ParentsBlock{
			ParentsType:  ParentsType(parentType),
			ParentsCount: parentsCount,
			References:   references,
		}
	}

	issuerPublicKey, err := ed25519.ParsePublicKey(marshalUtil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer public key of the message: %w", err)
	}
	issuingTime, err := marshalUtil.ReadTime()
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuing time of the message: %w", err)
	}
	msgSequenceNumber, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("failed to parse sequence number of the message: %w", err)
	}

	msgPayload, err := payload.FromMarshalUtil(marshalUtil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse payload of the message: %w", err)
	}

	nonce, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("failed to parse nonce of the message: %w", err)
	}
	signature, err := ed25519.ParseSignature(marshalUtil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signature of the message: %w", err)
	}

	// retrieve the number of bytes we processed
	readOffsetEnd := marshalUtil.ReadOffset()

	// store marshaled version as a copy
	msgBytes, err := marshalUtil.ReadBytes(readOffsetEnd-readOffsetStart, readOffsetStart)
	if err != nil {
		return nil, fmt.Errorf("error trying to copy raw source bytes: %w", err)
	}

	msg, err := newMessageWithValidation(version, parentsBlocks, issuingTime, issuerPublicKey, msgPayload, nonce, signature, msgSequenceNumber)
	if err != nil {
		return nil, err
	}

	msg.bytes = msgBytes

	return msg, nil
}

// MessageFromObjectStorage restores a Message from the ObjectStorage.
func MessageFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	// parse the message
	message, err := MessageFromMarshalUtil(marshalutil.New(data))
	if err != nil {
		err = fmt.Errorf("failed to parse message from object storage: %w", err)
		return
	}

	// parse the ID from they key
	id, err := ReferenceFromMarshalUtil(marshalutil.New(key))
	if err != nil {
		err = fmt.Errorf("failed to parse message ID from object storage: %w", err)
		return
	}
	message.id = &id

	// assign result
	result = message

	return
}

// VerifySignature verifies the signature of the message.
func (m *Message) VerifySignature() bool {
	msgBytes := m.Bytes()
	signature := m.Signature()

	contentLength := len(msgBytes) - len(signature)
	content := msgBytes[:contentLength]

	return m.issuerPublicKey.VerifySignature(content, signature)
}

// ID returns the id of the message which is made up of the content id and parent1/parent2 ids.
// This id can be used for merkle proofs.
func (m *Message) ID() (result MessageID) {
	m.idMutex.RLock()

	if m.id == nil {
		m.idMutex.RUnlock()

		m.idMutex.Lock()
		defer m.idMutex.Unlock()
		if m.id != nil {
			result = *m.id
			return
		}
		result = m.calculateID()
		m.id = &result
		return
	}

	result = *m.id
	m.idMutex.RUnlock()
	return
}

// IDBytes implements Element interface in scheduler NodeQueue that returns the MessageID of the message in bytes.
func (m *Message) IDBytes() []byte {
	return m.ID().Bytes()
}

// Version returns the message version.
func (m *Message) Version() uint8 {
	return m.version
}

// ParentsByType returns a slice of all parents of the desired type.
func (m *Message) ParentsByType(parentType ParentsType) MessageIDs {
	for _, parentBlock := range m.parentsBlocks {
		if parentBlock.ParentsType == parentType {
			return parentBlock.References
		}
	}
	return MessageIDs{}
}

// ForEachParent executes a consumer func for each parent.
func (m *Message) ForEachParent(consumer func(parent Parent)) {
	for _, parentBlock := range m.parentsBlocks {
		for _, parentID := range parentBlock.References {
			consumer(Parent{
				Type: parentBlock.ParentsType,
				ID:   parentID,
			})
		}
	}
}

// ForEachParentByType executes a consumer func for each strong parent.
func (m *Message) ForEachParentByType(parentType ParentsType, consumer func(parentMessageID MessageID)) {
	for _, parentID := range m.ParentsByType(parentType) {
		consumer(parentID)
	}
}

// ParentsCountByType returns the total parents count of this message.
func (m *Message) ParentsCountByType(parentType ParentsType) uint8 {
	return uint8(len(m.ParentsByType(parentType)))
}

// IssuerPublicKey returns the public key of the message issuer.
func (m *Message) IssuerPublicKey() ed25519.PublicKey {
	return m.issuerPublicKey
}

// IssuingTime returns the time when this message was created.
func (m *Message) IssuingTime() time.Time {
	return m.issuingTime
}

// SequenceNumber returns the sequence number of this message.
func (m *Message) SequenceNumber() uint64 {
	return m.sequenceNumber
}

// Payload returns the payload of the message.
func (m *Message) Payload() payload.Payload {
	return m.payload
}

// Nonce returns the nonce of the message.
func (m *Message) Nonce() uint64 {
	return m.nonce
}

// Signature returns the signature of the message.
func (m *Message) Signature() ed25519.Signature {
	return m.signature
}

// calculates the message's MessageID.
func (m *Message) calculateID() MessageID {
	return blake2b.Sum256(m.Bytes())
}

// Bytes returns the message in serialized byte form.
func (m *Message) Bytes() []byte {
	m.bytesMutex.Lock()
	defer m.bytesMutex.Unlock()
	if m.bytes != nil {
		return m.bytes
	}

	// marshal result
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(m.version)
	marshalUtil.WriteByte(m.parentsBlocksCount)

	for x := 0; x < int(m.parentsBlocksCount); x++ {
		parentBlock := m.parentsBlocks[x]
		marshalUtil.WriteByte(byte(parentBlock.ParentsType))
		marshalUtil.WriteByte(parentBlock.ParentsCount)
		sortedParents := sortParents(parentBlock.References)
		for _, parent := range sortedParents {
			marshalUtil.Write(parent)
		}
	}

	marshalUtil.Write(m.issuerPublicKey)
	marshalUtil.WriteTime(m.issuingTime)
	marshalUtil.WriteUint64(m.sequenceNumber)
	marshalUtil.Write(m.payload)
	marshalUtil.WriteUint64(m.nonce)
	marshalUtil.Write(m.signature)

	m.bytes = marshalUtil.Bytes()

	return m.bytes
}

// Size returns the message size in bytes.
func (m *Message) Size() int {
	return len(m.Bytes())
}

// ObjectStorageKey returns the key of the stored message object.
// This returns the bytes of the message ID.
func (m *Message) ObjectStorageKey() []byte {
	return m.ID().Bytes()
}

// ObjectStorageValue returns the value stored in object storage.
// This returns the bytes of message.
func (m *Message) ObjectStorageValue() []byte {
	return m.Bytes()
}

// Update updates the object with the values of another object.
// Since a Message is immutable, this function is not implemented and panics.
func (m *Message) Update(objectstorage.StorableObject) {
	panic("messages should never be overwritten and only stored once to optimize IO")
}

func (m *Message) String() string {
	builder := stringify.StructBuilder("Message", stringify.StructField("id", m.ID()))
	parents := m.ParentsByType(StrongParentType)
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("strongParent%d", index), parent.String()))
		}
	}
	parents = m.ParentsByType(WeakParentType)
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("weakParent%d", index), parent.String()))
		}
	}
	parents = m.ParentsByType(DislikeParentType)
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("dislikeParent%d", index), parent.String()))
		}
	}
	parents = m.ParentsByType(LikeParentType)
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("likeParent%d", index), parent.String()))
		}
	}
	builder.AddField(stringify.StructField("issuer", m.IssuerPublicKey()))
	builder.AddField(stringify.StructField("issuingTime", m.IssuingTime()))
	builder.AddField(stringify.StructField("sequenceNumber", m.SequenceNumber()))
	builder.AddField(stringify.StructField("payload", m.Payload()))
	builder.AddField(stringify.StructField("nonce", m.Nonce()))
	builder.AddField(stringify.StructField("signature", m.Signature()))
	return builder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Parent ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Parent is a parent that can be either strong or weak.
type Parent struct {
	ID   MessageID
	Type ParentsType
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMessage ////////////////////////////////////////////////////////////////////////////////////////////////

// CachedMessage defines a cached message.
// A wrapper for a cached object.
type CachedMessage struct {
	objectstorage.CachedObject
}

// Retain registers a new consumer for the cached message.
func (c *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{c.CachedObject.Retain()}
}

// Consume consumes the cached object and releases it when the callback is done.
// It returns true if the callback was called.
func (c *CachedMessage) Consume(consumer func(message *Message)) bool {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	})
}

// Unwrap returns the message wrapped by the cached message.
// If the wrapped object cannot be cast to a Message or has been deleted, it returns nil.
func (c *CachedMessage) Unwrap() *Message {
	untypedMessage := c.Get()
	if untypedMessage == nil {
		return nil
	}
	typeCastedMessage := untypedMessage.(*Message)
	if typeCastedMessage == nil || typeCastedMessage.IsDeleted() {
		return nil
	}
	return typeCastedMessage
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	objectstorage.StorableObjectFlags

	messageID          MessageID
	receivedTime       time.Time
	solid              bool
	solidificationTime time.Time
	structureDetails   *markers.StructureDetails
	branchID           ledgerstate.BranchID
	scheduled          bool
	scheduledTime      time.Time
	booked             bool
	bookedTime         time.Time
	eligible           bool
	invalid            bool
	finalized          bool
	finalizedTime      time.Time
	gradeOfFinality    gof.GradeOfFinality

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	structureDetailsMutex   sync.RWMutex
	branchIDMutex           sync.RWMutex
	scheduledMutex          sync.RWMutex
	scheduledTimeMutex      sync.RWMutex
	bookedMutex             sync.RWMutex
	bookedTimeMutex         sync.RWMutex
	eligibleMutex           sync.RWMutex
	invalidMutex            sync.RWMutex
	finalizedMutex          sync.RWMutex
	finalizedTimeMutex      sync.RWMutex
	gradeOfFinalityMutex    sync.RWMutex
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID MessageID) *MessageMetadata {
	return &MessageMetadata{
		messageID:    messageID,
		receivedTime: clock.SyncedTime(),
	}
}

// MessageMetadataFromBytes unmarshals the given bytes into a MessageMetadata.
func MessageMetadataFromBytes(bytes []byte) (result *MessageMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MessageMetadataFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MessageMetadataFromMarshalUtil parses a Message from the given MarshalUtil.
func MessageMetadataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *MessageMetadata, err error) {
	result = &MessageMetadata{}

	if result.messageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse message ID of message metadata: %w", err)
		return
	}
	if result.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse received time of message metadata: %w", err)
		return
	}
	if result.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse solidification time of message metadata: %w", err)
		return
	}
	if result.solid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse solid flag of message metadata: %w", err)
		return
	}
	if result.structureDetails, err = markers.StructureDetailsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse StructureDetails from MarshalUtil: %w", err)
		return
	}
	if result.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	if result.scheduled, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse scheduled flag of message metadata: %w", err)
		return
	}
	if result.scheduledTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse scheduled time of message metadata: %w", err)
		return
	}
	if result.booked, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse booked flag of message metadata: %w", err)
		return
	}
	if result.bookedTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse booked time of message metadata: %w", err)
		return
	}
	if result.eligible, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse eligble flag of message metadata: %w", err)
		return
	}
	if result.invalid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse invalid flag of message metadata: %w", err)
		return
	}
	if result.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse finalizedApprovalWeight flag of message metadata: %w", err)
		return
	}
	if result.finalizedTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse finalized time of message metadata: %w", err)
		return
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		err = fmt.Errorf("failed to parse grade of finality of message metadata: %w", err)
		return
	}
	result.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)

	return
}

// MessageMetadataFromObjectStorage restores a MessageMetadata object from the ObjectStorage.
func MessageMetadataFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MessageMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = fmt.Errorf("failed to parse message metadata from object storage: %w", err)
		return
	}

	return
}

// ID returns the MessageID of the Message that this MessageMetadata object belongs to.
func (m *MessageMetadata) ID() MessageID {
	return m.messageID
}

// ReceivedTime returns the time when the message was received.
func (m *MessageMetadata) ReceivedTime() time.Time {
	return m.receivedTime
}

// IsSolid returns true if the message represented by this metadata is solid. False otherwise.
func (m *MessageMetadata) IsSolid() (result bool) {
	m.solidMutex.RLock()
	defer m.solidMutex.RUnlock()
	result = m.solid

	return
}

// SetSolid sets the message associated with this metadata as solid.
// It returns true if the solid status is modified. False otherwise.
func (m *MessageMetadata) SetSolid(solid bool) (modified bool) {
	m.solidMutex.RLock()
	if m.solid != solid {
		m.solidMutex.RUnlock()

		m.solidMutex.Lock()
		if m.solid != solid {
			m.solid = solid
			if solid {
				m.solidificationTimeMutex.Lock()
				m.solidificationTime = clock.SyncedTime()
				m.solidificationTimeMutex.Unlock()
			}

			m.SetModified()

			modified = true
		}
		m.solidMutex.Unlock()
	} else {
		m.solidMutex.RUnlock()
	}

	return
}

// SolidificationTime returns the time when the message was marked to be solid.
func (m *MessageMetadata) SolidificationTime() time.Time {
	m.solidificationTimeMutex.RLock()
	defer m.solidificationTimeMutex.RUnlock()

	return m.solidificationTime
}

// SetStructureDetails sets the structureDetails of the message.
func (m *MessageMetadata) SetStructureDetails(structureDetails *markers.StructureDetails) (modified bool) {
	m.structureDetailsMutex.Lock()
	defer m.structureDetailsMutex.Unlock()

	if m.structureDetails != nil {
		return false
	}

	m.structureDetails = structureDetails

	m.SetModified()
	return true
}

// StructureDetails returns the structureDetails of the message.
func (m *MessageMetadata) StructureDetails() *markers.StructureDetails {
	m.structureDetailsMutex.RLock()
	defer m.structureDetailsMutex.RUnlock()

	return m.structureDetails
}

// SetBranchID sets the branch ID of the message.
func (m *MessageMetadata) SetBranchID(bID ledgerstate.BranchID) (modified bool) {
	m.branchIDMutex.Lock()
	defer m.branchIDMutex.Unlock()
	if m.branchID == bID {
		return
	}
	m.branchID = bID
	m.SetModified(true)
	modified = true
	return
}

// BranchID returns the branch ID of the message.
func (m *MessageMetadata) BranchID() ledgerstate.BranchID {
	m.branchIDMutex.RLock()
	defer m.branchIDMutex.RUnlock()
	return m.branchID
}

// IsEligible returns true if the message represented by this metadata is eligible. False otherwise.
func (m *MessageMetadata) IsEligible() (result bool) {
	m.eligibleMutex.RLock()
	defer m.eligibleMutex.RUnlock()
	result = m.eligible

	return
}

// SetScheduled sets the message associated with this metadata as scheduled.
// It returns true if the scheduled status is modified. False otherwise.
func (m *MessageMetadata) SetScheduled(scheduled bool) (modified bool) {
	m.scheduledMutex.Lock()
	defer m.scheduledMutex.Unlock()
	m.scheduledTimeMutex.Lock()
	defer m.scheduledTimeMutex.Unlock()

	if m.scheduled == scheduled {
		return false
	}

	m.scheduled = scheduled
	m.scheduledTime = clock.SyncedTime()
	m.SetModified()
	modified = true

	return
}

// Scheduled returns true if the message represented by this metadata was scheduled. False otherwise.
func (m *MessageMetadata) Scheduled() (result bool) {
	m.scheduledMutex.RLock()
	defer m.scheduledMutex.RUnlock()

	return m.scheduled
}

// ScheduledTime returns the time when the message represented by this metadata was scheduled.
func (m *MessageMetadata) ScheduledTime() time.Time {
	m.scheduledTimeMutex.RLock()
	defer m.scheduledTimeMutex.RUnlock()

	return m.scheduledTime
}

// SetBooked sets the message associated with this metadata as booked.
// It returns true if the booked status is modified. False otherwise.
func (m *MessageMetadata) SetBooked(booked bool) (modified bool) {
	m.bookedMutex.Lock()
	defer m.bookedMutex.Unlock()
	m.bookedTimeMutex.Lock()
	defer m.bookedTimeMutex.Unlock()

	if m.booked == booked {
		return false
	}

	m.booked = booked
	m.bookedTime = clock.SyncedTime()
	m.SetModified()
	modified = true

	return
}

// IsBooked returns true if the message represented by this metadata is booked. False otherwise.
func (m *MessageMetadata) IsBooked() (result bool) {
	m.bookedMutex.RLock()
	defer m.bookedMutex.RUnlock()
	result = m.booked

	return
}

// BookedTime returns the time when the message represented by this metadata was booked.
func (m *MessageMetadata) BookedTime() time.Time {
	m.bookedTimeMutex.RLock()
	defer m.bookedTimeMutex.RUnlock()

	return m.bookedTime
}

// SetEligible sets the message associated with this metadata as eligible.
// It returns true if the eligible status is modified. False otherwise.
func (m *MessageMetadata) SetEligible(eligible bool) (modified bool) {
	m.eligibleMutex.Lock()
	defer m.eligibleMutex.Unlock()

	if m.eligible == eligible {
		return false
	}

	m.eligible = eligible
	m.SetModified()
	modified = true

	return
}

// IsInvalid returns true if the message represented by this metadata is invalid. False otherwise.
func (m *MessageMetadata) IsInvalid() (result bool) {
	m.invalidMutex.RLock()
	defer m.invalidMutex.RUnlock()
	result = m.invalid

	return
}

// SetInvalid sets the message associated with this metadata as invalid.
// It returns true if the invalid status is modified. False otherwise.
func (m *MessageMetadata) SetInvalid(invalid bool) (modified bool) {
	m.invalidMutex.Lock()
	defer m.invalidMutex.Unlock()

	if m.invalid == invalid {
		return false
	}

	m.invalid = invalid
	m.SetModified()
	modified = true

	return
}

// SetFinalized sets the message associated with this metadata as finalized by approval weight.
// It returns true if the finalized status is modified. False otherwise.
func (m *MessageMetadata) SetFinalized(finalized bool) (modified bool) {
	m.finalizedMutex.Lock()
	m.finalizedTimeMutex.Lock()
	defer m.finalizedMutex.Unlock()
	defer m.finalizedTimeMutex.Unlock()

	if m.finalized == finalized {
		return false
	}

	m.finalized = finalized
	m.finalizedTime = clock.SyncedTime()
	m.SetModified()
	modified = true

	return
}

// IsFinalized returns true if the message represented by this metadata is finalized by approval weight.
// False otherwise.
func (m *MessageMetadata) IsFinalized() (result bool) {
	m.finalizedMutex.RLock()
	defer m.finalizedMutex.RUnlock()

	return m.finalized
}

// FinalizedTime returns the time when the message represented by this metadata was finalized.
func (m *MessageMetadata) FinalizedTime() time.Time {
	m.finalizedTimeMutex.RLock()
	defer m.finalizedTimeMutex.RUnlock()

	return m.finalizedTime
}

// SetGradeOfFinality sets the grade of finality associated with this metadata.
// It returns true if the grade of finality is modified. False otherwise.
func (m *MessageMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	m.gradeOfFinalityMutex.Lock()
	defer m.gradeOfFinalityMutex.Unlock()

	if m.gradeOfFinality == gradeOfFinality {
		return false
	}

	m.gradeOfFinality = gradeOfFinality
	m.SetModified()
	modified = true

	return
}

// GradeOfFinality returns the grade of finality.
func (m *MessageMetadata) GradeOfFinality() (result gof.GradeOfFinality) {
	m.gradeOfFinalityMutex.RLock()
	defer m.gradeOfFinalityMutex.RUnlock()

	return m.gradeOfFinality
}

// Bytes returns a marshaled version of the whole MessageMetadata object.
func (m *MessageMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// ObjectStorageKey returns the key of the stored message metadata object.
// This returns the bytes of the messageID.
func (m *MessageMetadata) ObjectStorageKey() []byte {
	return m.messageID.Bytes()
}

// ObjectStorageValue returns the value of the stored message metadata object.
// This includes the receivedTime, solidificationTime and solid status.
func (m *MessageMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(m.ReceivedTime()).
		WriteTime(m.SolidificationTime()).
		WriteBool(m.IsSolid()).
		Write(m.StructureDetails()).
		Write(m.BranchID()).
		WriteBool(m.Scheduled()).
		WriteTime(m.ScheduledTime()).
		WriteBool(m.IsBooked()).
		WriteTime(m.BookedTime()).
		WriteBool(m.IsEligible()).
		WriteBool(m.IsInvalid()).
		WriteBool(m.IsFinalized()).
		WriteTime(m.FinalizedTime()).
		WriteUint8(uint8(m.GradeOfFinality())).
		Bytes()
}

// Update updates the message metadata.
// This should never happen and will panic if attempted.
func (m *MessageMetadata) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// String returns a human readable version of the MessageMetadata.
func (m *MessageMetadata) String() string {
	return stringify.Struct("MessageMetadata",
		stringify.StructField("ID", m.messageID),
		stringify.StructField("receivedTime", m.ReceivedTime()),
		stringify.StructField("solid", m.IsSolid()),
		stringify.StructField("solidificationTime", m.SolidificationTime()),
		stringify.StructField("structureDetails", m.StructureDetails()),
		stringify.StructField("branchID", m.BranchID()),
		stringify.StructField("scheduled", m.Scheduled()),
		stringify.StructField("scheduledTime", m.ScheduledTime()),
		stringify.StructField("booked", m.IsBooked()),
		stringify.StructField("bookedTime", m.BookedTime()),
		stringify.StructField("eligible", m.IsEligible()),
		stringify.StructField("invalid", m.IsInvalid()),
		stringify.StructField("finalized", m.IsFinalized()),
		stringify.StructField("finalizedTime", m.FinalizedTime()),
		stringify.StructField("gradeOfFinality", m.GradeOfFinality()),
	)
}

var _ objectstorage.StorableObject = &MessageMetadata{}

// CachedMessageMetadata is a wrapper for stored cached object that represents a message metadata.
type CachedMessageMetadata struct {
	objectstorage.CachedObject
}

// ID returns the MessageID of the CachedMessageMetadata.
func (c *CachedMessageMetadata) ID() (messageID MessageID) {
	messageID, _, err := MessageIDFromBytes(c.Key())
	if err != nil {
		panic(err)
	}

	return
}

// Retain registers a new consumer for the cached message metadata.
func (c *CachedMessageMetadata) Retain() *CachedMessageMetadata {
	return &CachedMessageMetadata{c.CachedObject.Retain()}
}

// Unwrap returns the underlying stored message metadata wrapped by the CachedMessageMetadata.
// If the stored object cannot be cast to MessageMetadata or is deleted, it returns nil.
func (c *CachedMessageMetadata) Unwrap() *MessageMetadata {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}
	typedObject := untypedObject.(*MessageMetadata)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}
	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMessageMetadata) Consume(consumer func(messageMetadata *MessageMetadata), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MessageMetadata))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors /////////////////////////////tangle.m//////////////////////////////////////////////////////////////////////////
var (
	ErrNoStrongParents                    = errors.New("missing strong messages in first parent block")
	ErrBlocksNotOrderedByType             = errors.New("blocks should be ordered in ascending order according to their type")
	ErrBlockTypeIsUnknown                 = errors.Errorf("block types must range from %d-%d", 0, NumberOfBlockTypes-1)
	ErrParentsCountMismatch               = errors.New("number of parents in a message doesn't match parent count")
	ErrParentsOutOfRange                  = errors.Errorf("a block must have at least %d-%d parents", MinParentsCount, MaxParentsCount)
	ErrParentsNotLexicographicallyOrdered = errors.New("messages within blocks must be lexicographically ordered")
	ErrRepeatingBlockTypes                = errors.New("block types within a message must not repeat")
	ErrRepeatingReferencesInBlock         = errors.New("duplicate parents in a message block")
	ErrRepeatingMessagesAcrossBlocks      = errors.New("different blocks have repeating messages")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
