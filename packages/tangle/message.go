package tangle

import (
	"bytes"
	"fmt"
	"math/bits"
	"sort"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

const (
	// MessageVersion defines the version of the message structure.
	MessageVersion uint8 = 1

	// MaxMessageSize defines the maximum size of a message.
	MaxMessageSize = 64 * 1024

	// MessageIDLength defines the length of an MessageID.
	MessageIDLength = 32

	// StrongParent identifies a strong parent in the bitmask.
	StrongParent uint8 = 1

	// WeakParent identifies a weak parent in the bitmask.
	WeakParent uint8 = 0

	// MinParentsCount defines the minimum number of parents a message must have.
	MinParentsCount = 1

	// MaxParentsCount defines the maximum number of parents a message must have.
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

// MessageIDFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func MessageIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (MessageID, error) {
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

// String returns the base58 encode of the MessageID.
func (id MessageID) String() string {
	return base58.Encode(id[:])
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageIDs ///////////////////////////////////////////////////////////////////////////////////////////////////

// MessageIDs is a slice of MessageID.
type MessageIDs []MessageID

// ToStrings converts a slice of MessageIDs to a slice of strings.
func (ids MessageIDs) ToStrings() []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, id.String())
	}
	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Message //////////////////////////////////////////////////////////////////////////////////////////////////////

// Message represents the core message for the base layer Tangle.
type Message struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (get sent over the wire)
	version         uint8
	strongParents   []MessageID
	weakParents     []MessageID
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
func NewMessage(strongParents []MessageID, weakParents []MessageID, issuingTime time.Time, issuerPublicKey ed25519.PublicKey, sequenceNumber uint64, payload payload.Payload, nonce uint64, signature ed25519.Signature) (result *Message) {
	// remove duplicates, sort in ASC
	sortedStrongParents := sortParents(strongParents)
	sortedWeakParents := sortParents(weakParents)

	// syntactical validation
	parentsCount := len(sortedStrongParents) + len(sortedWeakParents)
	if parentsCount < MinParentsCount || parentsCount > MaxParentsCount {
		panic(fmt.Sprintf("amount of parents (%d) not in valid range (%d-%d)", parentsCount, MinParentsCount, MaxParentsCount))
	}

	if len(sortedStrongParents) < MinStrongParentsCount {
		panic(fmt.Sprintf("amount of strong parents (%d) failed to reach MinStrongParentsCount (%d)", len(strongParents), MinStrongParentsCount))
	}

	return &Message{
		version:         MessageVersion,
		strongParents:   sortedStrongParents,
		weakParents:     sortedWeakParents,
		issuerPublicKey: issuerPublicKey,
		issuingTime:     issuingTime,
		sequenceNumber:  sequenceNumber,
		payload:         payload,
		nonce:           nonce,
		signature:       signature,
	}
}

// filters and sorts given parents and returns a new slice with sorted parents
func sortParents(parents []MessageID) (sorted []MessageID) {
	seen := make(map[MessageID]types.Empty)
	sorted = make([]MessageID, 0, len(parents))

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
		err = xerrors.Errorf("consumed bytes %d not equal total bytes %d: %w", consumedBytes, len(bytes), cerrors.ErrParseBytesFailed)
	}
	return
}

// MessageFromMarshalUtil parses a message from the given marshal util.
func MessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Message, err error) {
	// determine read offset before starting to parse
	readOffsetStart := marshalUtil.ReadOffset()

	// parse information
	result = &Message{}
	if result.version, err = marshalUtil.ReadByte(); err != nil {
		err = xerrors.Errorf("failed to parse message version from MarshalUtil: %w", err)
		return
	}

	var parentsCount uint8
	if parentsCount, err = marshalUtil.ReadByte(); err != nil {
		err = xerrors.Errorf("failed to parse parents count from MarshalUtil: %w", err)
		return
	}
	if parentsCount < MinParentsCount || parentsCount > MaxParentsCount {
		err = xerrors.Errorf("parents count %d not allowed: %w", parentsCount, cerrors.ErrParseBytesFailed)
		return
	}

	var parentTypes uint8
	if parentTypes, err = marshalUtil.ReadByte(); err != nil {
		err = xerrors.Errorf("failed to parse parent types from MarshalUtil: %w", err)
		return
	}
	if bits.OnesCount8(parentTypes) < 1 {
		err = xerrors.Errorf("invalid parent types, no strong parent specified: %b", parentTypes)
		return
	}
	bitMask := bitmask.BitMask(parentTypes)

	var previousParent MessageID
	for i := 0; i < int(parentsCount); i++ {
		var parentID MessageID
		if parentID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse parent %d from MarshalUtil: %w", i, err)
			return
		}
		if bitMask.HasBit(uint(i)) {
			result.strongParents = append(result.strongParents, parentID)
		} else {
			result.weakParents = append(result.weakParents, parentID)
		}

		// verify that parents are sorted lexicographically ASC and unique
		// if parentID is EmptyMessageID, bytes.Compare returns 0 in the first iteration
		if bytes.Compare(previousParent.Bytes(), parentID.Bytes()) > 0 {
			err = xerrors.Errorf("parents not sorted lexicographically ascending: %w", cerrors.ErrParseBytesFailed)
			return
		}
		previousParent = parentID
	}

	if len(result.strongParents) < MinStrongParentsCount {
		err = xerrors.Errorf("strong parents count %d not allowed: %w", len(result.strongParents), cerrors.ErrParseBytesFailed)
		return
	}

	if result.issuerPublicKey, err = ed25519.ParsePublicKey(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse issuer public key of the message: %w", err)
		return
	}
	if result.issuingTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse issuing time of the message: %w", err)
		return
	}
	if result.sequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		err = fmt.Errorf("failed to parse sequence number of the message: %w", err)
		return
	}
	if result.payload, err = payload.FromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse payload of the message: %w", err)
		return
	}
	if result.nonce, err = marshalUtil.ReadUint64(); err != nil {
		err = fmt.Errorf("failed to parse nonce of the message: %w", err)
		return
	}
	if result.signature, err = ed25519.ParseSignature(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse signature of the message: %w", err)
		return
	}

	// retrieve the number of bytes we processed
	readOffsetEnd := marshalUtil.ReadOffset()

	// store marshaled version as a copy
	result.bytes, err = marshalUtil.ReadBytes(readOffsetEnd-readOffsetStart, readOffsetStart)
	if err != nil {
		err = fmt.Errorf("error trying to copy raw source bytes: %w", err)
		return
	}

	return
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
	id, err := MessageIDFromMarshalUtil(marshalutil.New(key))
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

// Version returns the message version.
func (m *Message) Version() uint8 {
	return m.version
}

// Parents returns a slice of all parents of the Message.
func (m *Message) Parents() (parents MessageIDs) {
	return append(append([]MessageID{}, m.strongParents...), m.weakParents...)
}

// StrongParents returns a slice of all strong parents of the message.
func (m *Message) StrongParents() MessageIDs {
	return m.strongParents
}

// WeakParents returns a slice of all weak parents of the message.
func (m *Message) WeakParents() MessageIDs {
	return m.weakParents
}

// ForEachParent executes a consumer func for each parent.
func (m *Message) ForEachParent(consumer func(parent Parent)) {
	for _, parentID := range m.strongParents {
		consumer(Parent{
			ID:   parentID,
			Type: StrongParent,
		})
	}
	for _, parentID := range m.weakParents {
		consumer(Parent{
			ID:   parentID,
			Type: WeakParent,
		})
	}
}

// ForEachStrongParent executes a consumer func for each strong parent.
func (m *Message) ForEachStrongParent(consumer func(parentMessageID MessageID)) {
	for _, parentID := range m.strongParents {
		consumer(parentID)
	}
}

// ForEachWeakParent executes a consumer func for each weak parent.
func (m *Message) ForEachWeakParent(consumer func(parentMessageID MessageID)) {
	for _, parentID := range m.weakParents {
		consumer(parentID)
	}
}

// ParentsCount returns the total parents count of this message.
func (m *Message) ParentsCount() uint8 {
	return uint8(len(m.strongParents) + len(m.weakParents))
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
	marshalUtil.WriteByte(m.ParentsCount())

	parents := make([]Parent, 0, m.ParentsCount())

	for _, parent := range m.strongParents {
		parents = append(parents, Parent{ID: parent, Type: StrongParent})
	}
	for _, parent := range m.weakParents {
		parents = append(parents, Parent{ID: parent, Type: WeakParent})
	}
	sort.Slice(parents, func(i, j int) bool {
		return bytes.Compare(parents[i].ID.Bytes(), parents[j].ID.Bytes()) < 0
	})
	var bitMask bitmask.BitMask
	for i, parent := range parents {
		if parent.Type == StrongParent {
			bitMask = bitMask.SetBit(uint(i))
		}
	}
	marshalUtil.WriteByte(byte(bitMask))
	for _, parent := range parents {
		marshalUtil.Write(parent.ID)
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
	for index, parent := range m.strongParents {
		builder.AddField(stringify.StructField(fmt.Sprintf("strongParent%d", index), parent.String()))
	}
	for index, parent := range m.weakParents {
		builder.AddField(stringify.StructField(fmt.Sprintf("weakParent%d", index), parent.String()))
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
	Type uint8
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
	timestampOpinion   TimestampOpinion
	booked             bool
	eligible           bool
	invalid            bool

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	structureDetailsMutex   sync.RWMutex
	branchIDMutex           sync.RWMutex
	timestampOpinionMutex   sync.RWMutex
	bookedMutex             sync.RWMutex
	eligibleMutex           sync.RWMutex
	invalidMutex            sync.RWMutex
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

	if result.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
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
		err = xerrors.Errorf("failed to parse StructureDetails from MarshalUtil: %w", err)
		return
	}
	if result.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	if result.timestampOpinion, err = TimestampOpinionFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse timestampOpinion of message metadata: %w", err)
		return
	}
	if result.eligible, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse eligble flag of message metadata: %w", err)
		return
	}
	if result.booked, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse booked flag of message metadata: %w", err)
		return
	}
	if result.invalid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse invalid flag of message metadata: %w", err)
		return
	}

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
func (m *MessageMetadata) SetBranchID(ID ledgerstate.BranchID) (modified bool) {
	m.branchIDMutex.Lock()
	defer m.branchIDMutex.Unlock()
	if m.branchID == ID {
		return
	}
	m.branchID = ID
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

// IsBooked returns true if the message represented by this metadata is booked. False otherwise.
func (m *MessageMetadata) IsBooked() (result bool) {
	m.bookedMutex.RLock()
	defer m.bookedMutex.RUnlock()
	result = m.booked

	return
}

// IsEligible returns true if the message represented by this metadata is eligible. False otherwise.
func (m *MessageMetadata) IsEligible() (result bool) {
	m.eligibleMutex.RLock()
	defer m.eligibleMutex.RUnlock()
	result = m.eligible

	return
}

// SetBooked sets the message associated with this metadata as booked.
// It returns true if the booked status is modified. False otherwise.
func (m *MessageMetadata) SetBooked(booked bool) (modified bool) {
	m.bookedMutex.Lock()
	defer m.bookedMutex.Unlock()

	if m.booked == booked {
		return false
	}

	m.booked = booked
	m.SetModified()
	modified = true

	return
}

// TimestampOpinion returns the timestampOpinion of the given message metadata.
func (m *MessageMetadata) TimestampOpinion() (timestampOpinion TimestampOpinion) {
	m.timestampOpinionMutex.RLock()
	defer m.timestampOpinionMutex.RUnlock()
	return m.timestampOpinion
}

// SetTimestampOpinion sets the timestampOpinion flag.
// It returns true if the timestampOpinion flag is modified. False otherwise.
func (m *MessageMetadata) SetTimestampOpinion(timestampOpinion TimestampOpinion) (modified bool) {
	m.timestampOpinionMutex.Lock()
	defer m.timestampOpinionMutex.Unlock()

	if m.timestampOpinion.Equal(timestampOpinion) {
		return false
	}

	m.timestampOpinion = timestampOpinion
	m.SetModified()
	return true
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
		WriteBytes(m.TimestampOpinion().Bytes()).
		WriteBool(m.IsEligible()).
		WriteBool(m.IsBooked()).
		WriteBool(m.IsInvalid()).
		Bytes()
}

// Update updates the message metadata.
// This should never happen and will panic if attempted.
func (m *MessageMetadata) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
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
