package tangle

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
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

	// MinParentsBlocksCount defines the minimum number of parents each parents block must have.
	MinParentsBlocksCount = 1

	// MaxParentsBlocksCount defines the maximum number of parents each parents block must have.
	MaxParentsBlocksCount = 4

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

// MessageIDs is a set of MessageIDs where every MessageID is stored only once.
type MessageIDs map[MessageID]types.Empty

// NewMessageIDs construct a new MessageID collection from the optional MessageIDs.
func NewMessageIDs(msgIDs ...MessageID) MessageIDs {
	m := make(MessageIDs)
	for _, msgID := range msgIDs {
		m[msgID] = types.Void
	}

	return m
}

// Slice converts the set of MessageIDs into a slice of MessageIDs.
func (m MessageIDs) Slice() []MessageID {
	ids := make([]MessageID, 0)
	for key := range m {
		ids = append(ids, key)
	}
	return ids
}

// Clone creates a copy of the MessageIDs.
func (m MessageIDs) Clone() (clonedMessageIDs MessageIDs) {
	clonedMessageIDs = make(MessageIDs)
	for key, value := range m {
		clonedMessageIDs[key] = value
	}
	return
}

// Add adds a MessageID to the collection and returns the collection to enable chaining.
func (m MessageIDs) Add(messageID MessageID) MessageIDs {
	m[messageID] = types.Void

	return m
}

// AddAll adds all MessageIDs to the collection and returns the collection to enable chaining.
func (m MessageIDs) AddAll(messageIDs MessageIDs) MessageIDs {
	for messageID := range messageIDs {
		m.Add(messageID)
	}

	return m
}

// Empty checks if MessageIDs is empty.
func (m MessageIDs) Empty() (empty bool) {
	return len(m) == 0
}

// Contains checks if the given target MessageID is part of the MessageIDs.
func (m MessageIDs) Contains(target MessageID) (contains bool) {
	_, contains = m[target]
	return
}

// Subtract removes all other from the collection and returns the collection to enable chaining.
func (m MessageIDs) Subtract(other MessageIDs) MessageIDs {
	for messageID := range other {
		delete(m, messageID)
	}

	return m
}

// First returns the first element in MessageIDs (not ordered). This method only makes sense if there is exactly one
// element in the collection.
func (m MessageIDs) First() MessageID {
	for messageID := range m {
		return messageID
	}
	return EmptyMessageID
}

// Base58 returns a string slice of base58 MessageID.
func (m MessageIDs) Base58() (result []string) {
	result = make([]string, 0, len(m))
	for id := range m {
		result = append(result, id.Base58())
	}

	return
}

// String returns a human-readable version of the MessageIDs.
func (m MessageIDs) String() string {
	if len(m) == 0 {
		return "MessageIDs{}"
	}

	result := "MessageIDs{\n"
	for messageID := range m {
		result += strings.Repeat(" ", stringify.INDENTATION_SIZE) + messageID.String() + ",\n"
	}
	result += "}"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Message //////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// LastValidBlockType counts StrongParents, WeakParents, ShallowLikeParents, ShallowDislikeParents.
	LastValidBlockType = ShallowDislikeParentType
)

// Message represents the core message for the base layer Tangle.
type Message struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (get sent over the wire)
	version         uint8
	parentsBlocks   []ParentsBlock
	issuerPublicKey ed25519.PublicKey
	issuingTime     time.Time
	sequenceNumber  uint64
	payload         payload.Payload
	nonce           uint64
	signature       ed25519.Signature

	// derived properties
	id         *MessageID
	idMutex    sync.RWMutex
	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewMessage creates a new message with the details provided by the issuer.
func NewMessage(references ParentMessageIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, msgPayload payload.Payload, nonce uint64, signature ed25519.Signature) (*Message, error) {
	// remove duplicates, sort in ASC
	sortedStrongParents := sortParents(references[StrongParentType])
	sortedWeakParents := sortParents(references[WeakParentType])
	sortedShallowDislikeParents := sortParents(references[ShallowDislikeParentType])
	sortedShallowLikeParents := sortParents(references[ShallowLikeParentType])

	weakParentsCount := len(sortedWeakParents)
	shallowDislikeParentsCount := len(sortedShallowDislikeParents)
	shallowLikeParentsCount := len(sortedShallowLikeParents)

	var parentsBlocks []ParentsBlock

	parentsBlocks = append(parentsBlocks, ParentsBlock{
		ParentsType: StrongParentType,
		References:  sortedStrongParents,
	})

	if weakParentsCount > 0 {
		parentsBlocks = append(parentsBlocks, ParentsBlock{
			ParentsType: WeakParentType,
			References:  sortedWeakParents,
		})
	}

	if shallowLikeParentsCount > 0 {
		parentsBlocks = append(parentsBlocks, ParentsBlock{
			ParentsType: ShallowLikeParentType,
			References:  sortedShallowLikeParents,
		})
	}

	if shallowDislikeParentsCount > 0 {
		parentsBlocks = append(parentsBlocks, ParentsBlock{
			ParentsType: ShallowDislikeParentType,
			References:  sortedShallowDislikeParents,
		})
	}

	return newMessageWithValidation(MessageVersion, parentsBlocks, issuingTime, issuerPublicKey, msgPayload, nonce, signature, sequenceNumber)
}

// newMessageWithValidation creates a new message while performing ths following syntactical checks:
// 1. A Strong Parents Block must exist.
// 2. Parents Block types cannot repeat.
// 3. Parent count per block 1 <= x <= 8.
// 4. Parents unique within block.
// 5. Parents lexicographically sorted within block.
// 6. A Parent(s) repetition is only allowed when it occurs across Strong and Like parents.
// 7. Blocks should be ordered by type in ascending order.
func newMessageWithValidation(version uint8, parentsBlocks []ParentsBlock, issuingTime time.Time,
	issuerPublicKey ed25519.PublicKey, msgPayload payload.Payload, nonce uint64,
	signature ed25519.Signature, sequenceNumber uint64) (result *Message, err error) {
	// Validate strong parent block
	if len(parentsBlocks) == 0 || parentsBlocks[0].ParentsType != StrongParentType ||
		len(parentsBlocks[0].References) < MinStrongParentsCount {
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
		if parentsBlocks[i+1].ParentsType > LastValidBlockType {
			return nil, ErrBlockTypeIsUnknown
		}
	}

	// 1. Parent Count is correct for each block
	// 2. Number of parents in eac block is in range
	// 3. Parents are lexicographically ordered with no repetitions
	for _, block := range parentsBlocks {
		if len(block.References) > MaxParentsCount || len(block.References) < MinParentsCount {
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

	if areReferencesConflictingAcrossBlocks(parentsBlocks) {
		return nil, ErrConflictingReferenceAcrossBlocks
	}

	return &Message{
		version:         version,
		parentsBlocks:   parentsBlocks,
		issuerPublicKey: issuerPublicKey,
		issuingTime:     issuingTime,
		sequenceNumber:  sequenceNumber,
		payload:         msgPayload,
		nonce:           nonce,
		signature:       signature,
	}, nil
}

// validate messagesIDs are unique across blocks
// there may be repetition across strong and like parents.
func areReferencesConflictingAcrossBlocks(parentsBlocks []ParentsBlock) bool {
	additiveParents := MessageIDs{}
	subtractiveParents := MessageIDs{}

	for _, parentBlock := range parentsBlocks {
		for _, parent := range parentBlock.References {
			if parentBlock.ParentsType == WeakParentType || parentBlock.ParentsType == ShallowLikeParentType {
				additiveParents[parent] = types.Void
			} else if parentBlock.ParentsType == ShallowDislikeParentType {
				subtractiveParents[parent] = types.Void
			}
		}
	}

	for parent := range subtractiveParents {
		if _, exists := additiveParents[parent]; exists {
			return true
		}
	}

	return false
}

// filters and sorts given parents and returns a new slice with sorted parents
func sortParents(parents MessageIDs) (sorted []MessageID) {
	sorted = parents.Slice()

	// sort parents
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].Bytes(), sorted[j].Bytes()) < 0
	})

	return
}

// FromObjectStorage parses the given key and bytes into a message.
func (m *Message) FromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {

	// parse the message
	message, err := m.FromBytes(data)
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

// FromBytes parses the given bytes into a message.
func (m *Message) FromBytes(bytes []byte) (message *Message, err error) {
	marshalUtil := marshalutil.New(bytes)
	message, err = m.FromMarshalUtil(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes := marshalUtil.ReadOffset()

	if len(bytes) != consumedBytes {
		err = errors.Errorf("consumed bytes %d not equal total bytes %d: %w", consumedBytes, len(bytes), cerrors.ErrParseBytesFailed)
	}
	return
}

// FromMarshalUtil parses a message from the given marshal util.
func (m *Message) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*Message, error) {
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
		references := make([]MessageID, parentsCount)
		for j := 0; j < int(parentsCount); j++ {
			if references[j], err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
				return nil, errors.Errorf("failed to parse parent %d-%d from MarshalUtil: %w", i, j, err)
			}
		}
		parentsBlocks[i] = ParentsBlock{
			ParentsType: ParentsType(parentType),
			References:  references,
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
			return NewMessageIDs(parentBlock.References...)
		}
	}
	return NewMessageIDs()
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
func (m *Message) ForEachParentByType(parentType ParentsType, consumer func(parentMessageID MessageID) bool) {
	for parentID := range m.ParentsByType(parentType) {
		if !consumer(parentID) {
			return
		}
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
	marshalUtil.WriteByte(byte(len(m.parentsBlocks)))

	for x := 0; x < len(m.parentsBlocks); x++ {
		parentBlock := m.parentsBlocks[x]
		marshalUtil.WriteByte(byte(parentBlock.ParentsType))
		marshalUtil.WriteByte(byte(len(parentBlock.References)))
		sortedParents := sortParents(NewMessageIDs(parentBlock.References...))
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

func (m *Message) String() string {
	builder := stringify.StructBuilder("Message", stringify.StructField("id", m.ID()))
	parents := sortParents(m.ParentsByType(StrongParentType))
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("strongParent%d", index), parent.String()))
		}
	}
	parents = sortParents(m.ParentsByType(WeakParentType))
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("weakParent%d", index), parent.String()))
		}
	}
	parents = sortParents(m.ParentsByType(ShallowDislikeParentType))
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("shallowdislikeParent%d", index), parent.String()))
		}
	}
	parents = sortParents(m.ParentsByType(ShallowLikeParentType))
	if len(parents) > 0 {
		for index, parent := range parents {
			builder.AddField(stringify.StructField(fmt.Sprintf("shallowlikeParent%d", index), parent.String()))
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

var _ objectstorage.StorableObject = new(Message)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Parent ///////////////////////////////////////////////////////////////////////////////////////////////////////

// ParentsType is a type that defines the type of the parent.
type ParentsType uint8

const (
	// UndefinedParentType is the undefined parent.
	UndefinedParentType ParentsType = iota
	// StrongParentType is the ParentsType for a strong parent.
	StrongParentType
	// WeakParentType is the ParentsType for a weak parent.
	WeakParentType
	// ShallowLikeParentType is the ParentsType for the shallow like parent.
	ShallowLikeParentType
	// ShallowDislikeParentType is the ParentsType for a shallow dislike parent.
	ShallowDislikeParentType
)

// String returns string representation of ParentsType.
func (bp ParentsType) String() string {
	return fmt.Sprintf("ParentType(%s)", []string{"Undefined", "Strong", "Weak", "Shallow Like", "Shallow Dislike"}[bp])
}

// Parent is a parent that can be either strong or weak.
type Parent struct {
	ID   MessageID
	Type ParentsType
}

// ParentsBlock is the container for parents in a Message.
type ParentsBlock struct {
	ParentsType
	References []MessageID
}

// ParentMessageIDs is a map of ParentType to MessageIDs.
type ParentMessageIDs map[ParentsType]MessageIDs

// NewParentMessageIDs constructs a new ParentMessageIDs.
func NewParentMessageIDs() ParentMessageIDs {
	p := make(ParentMessageIDs)
	for _, parentType := range []ParentsType{StrongParentType, WeakParentType, ShallowLikeParentType, ShallowDislikeParentType} {
		p[parentType] = NewMessageIDs()
	}

	return p
}

// AddStrong adds a strong parent to the map.
func (p ParentMessageIDs) AddStrong(messageID MessageID) ParentMessageIDs {
	return p.Add(StrongParentType, messageID)
}

// Add adds a parent to the map.
func (p ParentMessageIDs) Add(parentType ParentsType, messageID MessageID) ParentMessageIDs {
	p[parentType].Add(messageID)
	return p
}

// AddAll adds a collection of parents to the map.
func (p ParentMessageIDs) AddAll(parentType ParentsType, messageIDs MessageIDs) ParentMessageIDs {
	p[parentType].AddAll(messageIDs)
	return p
}

// Clone returns a copy of map.
func (p ParentMessageIDs) Clone() ParentMessageIDs {
	pCloned := NewParentMessageIDs()
	for parentType, messageIDs := range p {
		pCloned.AddAll(parentType, messageIDs)
	}
	return pCloned
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	objectstorage.StorableObjectFlags

	messageID           MessageID
	receivedTime        time.Time
	solid               bool
	solidificationTime  time.Time
	structureDetails    *markers.StructureDetails
	addedBranchIDs      ledgerstate.BranchIDs
	subtractedBranchIDs ledgerstate.BranchIDs
	scheduled           bool
	scheduledTime       time.Time
	discardedTime       time.Time
	queuedTime          time.Time
	booked              bool
	bookedTime          time.Time
	objectivelyInvalid  bool
	subjectivelyInvalid bool
	gradeOfFinality     gof.GradeOfFinality
	gradeOfFinalityTime time.Time

	solidMutex               sync.RWMutex
	solidificationTimeMutex  sync.RWMutex
	structureDetailsMutex    sync.RWMutex
	addedBranchIDsMutex      sync.RWMutex
	subtractedBranchIDsMutex sync.RWMutex
	scheduledMutex           sync.RWMutex
	scheduledTimeMutex       sync.RWMutex
	discardedTimeMutex       sync.RWMutex
	queuedTimeMutex          sync.RWMutex
	bookedMutex              sync.RWMutex
	bookedTimeMutex          sync.RWMutex
	invalidMutex             sync.RWMutex
	gradeOfFinalityMutex     sync.RWMutex
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID MessageID) *MessageMetadata {
	return &MessageMetadata{
		messageID:           messageID,
		receivedTime:        clock.SyncedTime(),
		addedBranchIDs:      ledgerstate.NewBranchIDs(),
		subtractedBranchIDs: ledgerstate.NewBranchIDs(),
	}
}

// FromObjectStorage creates an MessageMetadata from sequences of key and bytes.
func (m *MessageMetadata) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := m.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = fmt.Errorf("failed to parse message metadata from object storage: %w", err)
	}
	return result, err
}

// FromBytes unmarshals the given bytes into a MessageMetadata.
func (m *MessageMetadata) FromBytes(bytes []byte) (result *MessageMetadata, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = m.FromMarshalUtil(marshalUtil)

	return
}

// FromMarshalUtil parses a Message from the given MarshalUtil.
func (m *MessageMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (messageMetadata *MessageMetadata, err error) {
	if messageMetadata = m; messageMetadata == nil {
		messageMetadata = new(MessageMetadata)
	}

	if messageMetadata.messageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse message ID of message metadata: %w", err)
		return
	}
	if messageMetadata.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse received time of message metadata: %w", err)
		return
	}
	if messageMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse solidification time of message metadata: %w", err)
		return
	}
	if messageMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse solid flag of message metadata: %w", err)
		return
	}
	if messageMetadata.structureDetails, err = markers.StructureDetailsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse StructureDetails from MarshalUtil: %w", err)
		return
	}
	if messageMetadata.addedBranchIDs, err = ledgerstate.BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse added BranchIDs from MarshalUtil: %w", err)
		return
	}
	if messageMetadata.subtractedBranchIDs, err = ledgerstate.BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse subtracted BranchIDs from MarshalUtil: %w", err)
		return
	}
	if messageMetadata.scheduled, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse scheduled flag of message metadata: %w", err)
		return
	}
	if messageMetadata.scheduledTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse scheduled time of message metadata: %w", err)
		return
	}
	if messageMetadata.booked, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse booked flag of message metadata: %w", err)
		return
	}
	if messageMetadata.bookedTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse booked time of message metadata: %w", err)
		return
	}
	if messageMetadata.objectivelyInvalid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse invalid flag of message metadata: %w", err)
		return
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		err = fmt.Errorf("failed to parse grade of finality of message metadata: %w", err)
		return
	}
	messageMetadata.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if messageMetadata.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse gradeOfFinality time of message metadata: %w", err)
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

// SetAddedBranchIDs sets the BranchIDs of the added Branches.
func (m *MessageMetadata) SetAddedBranchIDs(addedBranchIDs ledgerstate.BranchIDs) (modified bool) {
	m.addedBranchIDsMutex.Lock()
	defer m.addedBranchIDsMutex.Unlock()

	if m.addedBranchIDs.Equals(addedBranchIDs) {
		return false
	}

	m.addedBranchIDs = addedBranchIDs.Clone()
	m.SetModified(true)
	modified = true

	return
}

// AddBranchID sets the BranchIDs of the added Branches.
func (m *MessageMetadata) AddBranchID(branchID ledgerstate.BranchID) (modified bool) {
	m.addedBranchIDsMutex.Lock()
	defer m.addedBranchIDsMutex.Unlock()

	if m.addedBranchIDs.Contains(branchID) {
		return
	}

	m.addedBranchIDs.Add(branchID)
	m.SetModified(true)
	return true
}

// AddedBranchIDs returns the BranchIDs of the added Branches of the Message.
func (m *MessageMetadata) AddedBranchIDs() ledgerstate.BranchIDs {
	m.addedBranchIDsMutex.RLock()
	defer m.addedBranchIDsMutex.RUnlock()

	return m.addedBranchIDs.Clone()
}

// SetSubtractedBranchIDs sets the BranchIDs of the subtracted Branches.
func (m *MessageMetadata) SetSubtractedBranchIDs(subtractedBranchIDs ledgerstate.BranchIDs) (modified bool) {
	m.subtractedBranchIDsMutex.Lock()
	defer m.subtractedBranchIDsMutex.Unlock()

	if m.subtractedBranchIDs.Equals(subtractedBranchIDs) {
		return false
	}

	m.subtractedBranchIDs = subtractedBranchIDs.Clone()
	m.SetModified(true)
	modified = true

	return
}

// SubtractedBranchIDs returns the BranchIDs of the subtracted Branches of the Message.
func (m *MessageMetadata) SubtractedBranchIDs() ledgerstate.BranchIDs {
	m.subtractedBranchIDsMutex.RLock()
	defer m.subtractedBranchIDsMutex.RUnlock()

	return m.subtractedBranchIDs.Clone()
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

// SetDiscardedTime add the discarded time of a message to the metadata.
func (m *MessageMetadata) SetDiscardedTime(discardedTime time.Time) {
	m.discardedTimeMutex.Lock()
	defer m.discardedTimeMutex.Unlock()

	m.discardedTime = discardedTime
}

// DiscardedTime returns when the message was discarded.
func (m *MessageMetadata) DiscardedTime() time.Time {
	m.discardedTimeMutex.RLock()
	defer m.discardedTimeMutex.RUnlock()

	return m.discardedTime
}

// QueuedTime returns the time a message entered the scheduling queue.
func (m *MessageMetadata) QueuedTime() time.Time {
	m.queuedTimeMutex.RLock()
	defer m.queuedTimeMutex.RUnlock()

	return m.queuedTime
}

// SetQueuedTime records the time the message entered the scheduler queue.
func (m *MessageMetadata) SetQueuedTime(queuedTime time.Time) {
	m.queuedTimeMutex.Lock()
	defer m.queuedTimeMutex.Unlock()

	m.queuedTime = queuedTime
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

// IsObjectivelyInvalid returns true if the message represented by this metadata is objectively invalid.
func (m *MessageMetadata) IsObjectivelyInvalid() (result bool) {
	m.invalidMutex.RLock()
	defer m.invalidMutex.RUnlock()
	result = m.objectivelyInvalid

	return
}

// SetObjectivelyInvalid sets the message associated with this metadata as objectively invalid - it returns true if the
// status was changed.
func (m *MessageMetadata) SetObjectivelyInvalid(invalid bool) (modified bool) {
	m.invalidMutex.Lock()
	defer m.invalidMutex.Unlock()

	if m.objectivelyInvalid == invalid {
		return false
	}

	m.objectivelyInvalid = invalid
	m.SetModified()
	modified = true

	return
}

// IsSubjectivelyInvalid returns true if the message represented by this metadata is subjectively invalid.
func (m *MessageMetadata) IsSubjectivelyInvalid() (result bool) {
	m.invalidMutex.RLock()
	defer m.invalidMutex.RUnlock()
	result = m.subjectivelyInvalid

	return
}

// SetSubjectivelyInvalid sets the message associated with this metadata as subjectively invalid - it returns true if
// the status was changed.
func (m *MessageMetadata) SetSubjectivelyInvalid(invalid bool) (modified bool) {
	m.invalidMutex.Lock()
	defer m.invalidMutex.Unlock()

	if m.subjectivelyInvalid == invalid {
		return false
	}

	m.subjectivelyInvalid = invalid
	m.SetModified()
	modified = true

	return
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
	m.gradeOfFinalityTime = clock.SyncedTime()
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

// GradeOfFinalityTime returns the time the grade of finality was set.
func (m *MessageMetadata) GradeOfFinalityTime() time.Time {
	m.gradeOfFinalityMutex.RLock()
	defer m.gradeOfFinalityMutex.RUnlock()

	return m.gradeOfFinalityTime
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
		Write(m.AddedBranchIDs()).
		Write(m.SubtractedBranchIDs()).
		WriteBool(m.Scheduled()).
		WriteTime(m.ScheduledTime()).
		WriteBool(m.IsBooked()).
		WriteTime(m.BookedTime()).
		WriteBool(m.IsObjectivelyInvalid()).
		WriteUint8(uint8(m.GradeOfFinality())).
		WriteTime(m.GradeOfFinalityTime()).
		Bytes()
}

// String returns a human readable version of the MessageMetadata.
func (m *MessageMetadata) String() string {
	return stringify.Struct("MessageMetadata",
		stringify.StructField("ID", m.messageID),
		stringify.StructField("receivedTime", m.ReceivedTime()),
		stringify.StructField("solid", m.IsSolid()),
		stringify.StructField("solidificationTime", m.SolidificationTime()),
		stringify.StructField("structureDetails", m.StructureDetails()),
		stringify.StructField("addedBranchIDs", m.AddedBranchIDs()),
		stringify.StructField("subtractedBranchIDs", m.SubtractedBranchIDs()),
		stringify.StructField("scheduled", m.Scheduled()),
		stringify.StructField("scheduledTime", m.ScheduledTime()),
		stringify.StructField("discardedTime", m.DiscardedTime()),
		stringify.StructField("queuedTime", m.QueuedTime()),
		stringify.StructField("booked", m.IsBooked()),
		stringify.StructField("bookedTime", m.BookedTime()),
		stringify.StructField("objectivelyInvalid", m.IsObjectivelyInvalid()),
		stringify.StructField("subjectivelyInvalid", m.IsSubjectivelyInvalid()),
		stringify.StructField("gradeOfFinality", m.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", m.GradeOfFinalityTime()),
	)
}

var _ objectstorage.StorableObject = new(MessageMetadata)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// ErrNoStrongParents is triggered if there no strong parents.
	ErrNoStrongParents = errors.New("missing strong messages in first parent block")
	// ErrBlocksNotOrderedByType is triggered when the blocks are not ordered by their type.
	ErrBlocksNotOrderedByType = errors.New("blocks should be ordered in ascending order according to their type")
	// ErrBlockTypeIsUnknown is triggered when the block type is unknown.
	ErrBlockTypeIsUnknown = errors.Errorf("block types must range from %d-%d", 1, LastValidBlockType-1)
	// ErrParentsOutOfRange is triggered when a block is out of range.
	ErrParentsOutOfRange = errors.Errorf("a block must have at least %d-%d parents", MinParentsCount, MaxParentsCount)
	// ErrParentsNotLexicographicallyOrdered is triggred when parents are not lexicographically ordered.
	ErrParentsNotLexicographicallyOrdered = errors.New("messages within blocks must be lexicographically ordered")
	// ErrRepeatingBlockTypes is triggered if there are repeating block types in the message.
	ErrRepeatingBlockTypes = errors.New("block types within a message must not repeat")
	// ErrRepeatingReferencesInBlock is triggered if there are duplicate parents in a message block.
	ErrRepeatingReferencesInBlock = errors.New("duplicate parents in a message block")
	// ErrConflictingReferenceAcrossBlocks is triggered if there conflicting references across blocks.
	ErrConflictingReferenceAcrossBlocks = errors.New("different blocks have conflicting references")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
