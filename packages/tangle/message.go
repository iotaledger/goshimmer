package tangle

import (
	"bytes"
	"context"
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
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/serix"
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

func init() {
	messageIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsCount,
		Max:            MaxParentsCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
	}
	err := serix.DefaultAPI.RegisterTypeSettings(MessageIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(messageIDsArrayRules))

	if err != nil {
		panic(fmt.Errorf("error registering MessageIDs type settings: %w", err))
	}
	parentsMessageIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsBlocksCount,
		Max:            MaxParentsBlocksCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
		UniquenessSliceFunc: func(next []byte) []byte {
			// return first byte which indicates the parent type
			return next[:1]
		},
	}
	err = serix.DefaultAPI.RegisterTypeSettings(ParentMessageIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(parentsMessageIDsArrayRules))
	if err != nil {
		panic(fmt.Errorf("error registering ParentMessageIDs type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterValidators(ParentMessageIDs{}, validateParentMessageIDsBytes, validateParentMessageIDs)

	if err != nil {
		panic(fmt.Errorf("error registering ParentMessageIDs validators: %w", err))
	}
}

func validateParentMessageIDs(_ context.Context, parents ParentMessageIDs) (err error) {
	// Validate strong parent block
	if strongParents, strongParentsExist := parents[StrongParentType]; len(parents) == 0 || !strongParentsExist ||
		len(strongParents) < MinStrongParentsCount {
		return ErrNoStrongParents
	}
	for parentsType, _ := range parents {
		if parentsType > LastValidBlockType {
			return ErrBlockTypeIsUnknown
		}
	}
	if areReferencesConflictingAcrossBlocks(parents) {
		return ErrConflictingReferenceAcrossBlocks
	}
	return nil
}

// validate messagesIDs are unique across blocks
// there may be repetition across strong and like parents.
func areReferencesConflictingAcrossBlocks(parentsBlocks map[ParentsType]MessageIDs) bool {
	additiveParents := NewMessageIDs()
	subtractiveParents := NewMessageIDs()

	for parentsType, parentBlockReferences := range parentsBlocks {
		for _, parent := range parentBlockReferences.Slice() {
			if parentsType == WeakParentType || parentsType == ShallowLikeParentType {
				additiveParents.Add(parent)
			} else if parentsType == ShallowDislikeParentType {
				subtractiveParents.Add(parent)
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

func validateParentMessageIDsBytes(_ context.Context, _ []byte) (err error) {
	return
}

const (
	// MessageVersion defines the Version of the message structure.
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
func MessageIDFromBytes(data []byte) (result MessageID, consumedBytes int, err error) {
	// check arguments
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, &result, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse MessageID: %w", err)
		return
	}
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

// Base58 returns a base58 encoded Version of the MessageID.
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
// readable string instead of the base58 encoded Version of itself.
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

// String returns a human-readable Version of the MessageIDs.
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
	messageInner `serix:"0"`
}
type messageInner struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (get sent over the wire)
	Version         uint8             `serix:"0"`
	Parents         ParentMessageIDs  `serix:"1"`
	IssuerPublicKey ed25519.PublicKey `serix:"2"`
	IssuingTime     time.Time         `serix:"3"`
	SequenceNumber  uint64            `serix:"4"`
	Payload         payload.Payload   `serix:"5,optional"`
	Nonce           uint64            `serix:"6"`
	Signature       ed25519.Signature `serix:"7"`

	// derived properties
	id         *MessageID
	idMutex    sync.RWMutex
	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewMessage creates a new message with the details provided by the issuer.
func NewMessage(references ParentMessageIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, msgPayload payload.Payload, nonce uint64, signature ed25519.Signature, versionOpt ...uint8) (*Message, error) {
	version := MessageVersion
	if len(versionOpt) == 1 {
		version = versionOpt[0]
	}
	msg := &Message{messageInner{
		Version:         version,
		Parents:         references,
		IssuerPublicKey: issuerPublicKey,
		IssuingTime:     issuingTime,
		SequenceNumber:  sequenceNumber,
		Payload:         msgPayload,
		Nonce:           nonce,
		Signature:       signature,
	}}

	return msg, nil
}

// newMessageWithValidation creates a new message while performing ths following syntactical checks:
// 1. A Strong Parents Block must exist.
// 2. Parents Block types cannot repeat.
// 3. Parent count per block 1 <= x <= 8.
// 4. Parents unique within block.
// 5. Parents lexicographically sorted within block.
// 7. Blocks should be ordered by type in ascending order.

// 6. A Parent(s) repetition is only allowed when it occurs across Strong and Like parents.
func newMessageWithValidation(references ParentMessageIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, msgPayload payload.Payload, nonce uint64, signature ed25519.Signature, version ...uint8) (result *Message, err error) {
	msg, _ := NewMessage(references, issuingTime, issuerPublicKey, sequenceNumber, msgPayload, nonce, signature, version...)

	_, err = serix.DefaultAPI.Encode(context.Background(), msg, serix.WithValidation())
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// FromObjectStorage creates a Message from sequences of key and bytes.
func (m *Message) FromObjectStorage(key, value []byte) (result objectstorage.StorableObject, err error) {
	// parse the message
	message, err := m.FromBytes(value)
	if err != nil {
		err = fmt.Errorf("failed to parse message from object storage: %w", err)
		return
	}
	messageID := new(MessageID)
	_, err = serix.DefaultAPI.Decode(context.Background(), key, messageID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Message.id: %w", err)
		return
	}
	message.messageInner.id = messageID
	result = message

	return
}

// FromBytes unmarshals a Transaction from a sequence of bytes.
func (m *Message) FromBytes(data []byte) (*Message, error) {
	msg := new(Message)
	if m != nil {
		msg = m
	}
	consumedBytes, err := serix.DefaultAPI.Decode(context.Background(), data, msg, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Message: %w", err)
		return msg, err
	}

	if len(data) != consumedBytes {
		err = errors.Errorf("consumed bytes %d not equal total bytes %d: %w", consumedBytes, len(data), cerrors.ErrParseBytesFailed)
	}

	msgPayload := msg.Payload()
	if msgPayload != nil && msgPayload.Type() == ledgerstate.TransactionType {
		transaction := msgPayload.(*ledgerstate.Transaction)

		ledgerstate.SetOutputID(transaction.Essence(), transaction.ID())
	}

	return msg, err
}

// VerifySignature verifies the Signature of the message.
func (m *Message) VerifySignature() bool {
	msgBytes := m.Bytes()
	signature := m.Signature()

	contentLength := len(msgBytes) - len(signature)
	content := msgBytes[:contentLength]

	return m.messageInner.IssuerPublicKey.VerifySignature(content, signature)
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

// Version returns the message Version.
func (m *Message) Version() uint8 {
	return m.messageInner.Version
}

// ParentsByType returns a slice of all parents of the desired type.
func (m *Message) ParentsByType(parentType ParentsType) MessageIDs {
	if parents, ok := m.Parents[parentType]; ok {
		return parents
	}
	return NewMessageIDs()
}

// ForEachParent executes a consumer func for each parent.
func (m *Message) ForEachParent(consumer func(parent Parent)) {
	for parentType, parents := range m.Parents {
		for parentID := range parents {
			consumer(Parent{
				Type: parentType,
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
	return m.messageInner.IssuerPublicKey
}

// IssuingTime returns the time when this message was created.
func (m *Message) IssuingTime() time.Time {
	return m.messageInner.IssuingTime
}

// SequenceNumber returns the sequence number of this message.
func (m *Message) SequenceNumber() uint64 {
	return m.messageInner.SequenceNumber
}

// Payload returns the Payload of the message.
func (m *Message) Payload() payload.Payload {
	return m.messageInner.Payload
}

// Nonce returns the Nonce of the message.
func (m *Message) Nonce() uint64 {
	return m.messageInner.Nonce
}

// Signature returns the Signature of the message.
func (m *Message) Signature() ed25519.Signature {
	return m.messageInner.Signature
}

// calculates the message's MessageID.
func (m *Message) calculateID() MessageID {
	return blake2b.Sum256(m.Bytes())
}

// Bytes returns a marshaled version of the Transaction.
func (m *Message) Bytes() []byte {
	m.bytesMutex.Lock()
	defer m.bytesMutex.Unlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// Size returns the message size in bytes.
func (m *Message) Size() int {
	return len(m.Bytes())
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *Message) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m.ID(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (m *Message) ObjectStorageValue() []byte {
	m.bytesMutex.Lock()
	defer m.bytesMutex.Unlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

func (m *Message) String() string {
	builder := stringify.StructBuilder("Message", stringify.StructField("id", m.ID()))

	for index, parent := range sortParents(m.ParentsByType(StrongParentType)) {
		builder.AddField(stringify.StructField(fmt.Sprintf("strongParent%d", index), parent.String()))
	}
	for index, parent := range sortParents(m.ParentsByType(WeakParentType)) {
		builder.AddField(stringify.StructField(fmt.Sprintf("weakParent%d", index), parent.String()))
	}
	for index, parent := range sortParents(m.ParentsByType(ShallowDislikeParentType)) {
		builder.AddField(stringify.StructField(fmt.Sprintf("shallowdislikeParent%d", index), parent.String()))
	}
	for index, parent := range sortParents(m.ParentsByType(ShallowLikeParentType)) {
		builder.AddField(stringify.StructField(fmt.Sprintf("shallowlikeParent%d", index), parent.String()))
	}

	builder.AddField(stringify.StructField("Issuer", m.IssuerPublicKey()))
	builder.AddField(stringify.StructField("IssuingTime", m.IssuingTime()))
	builder.AddField(stringify.StructField("SequenceNumber", m.SequenceNumber()))
	builder.AddField(stringify.StructField("Payload", m.Payload()))
	builder.AddField(stringify.StructField("Nonce", m.Nonce()))
	builder.AddField(stringify.StructField("Signature", m.Signature()))
	return builder.String()
}

// sorts given parents and returns a new slice with sorted parents
func sortParents(parents MessageIDs) (sorted []MessageID) {
	sorted = parents.Slice()

	// sort parents
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].Bytes(), sorted[j].Bytes()) < 0
	})

	return
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

// ParentMessageIDs is a map of ParentType to MessageIDs.
type ParentMessageIDs map[ParentsType]MessageIDs

// NewParentMessageIDs constructs a new ParentMessageIDs.
func NewParentMessageIDs() ParentMessageIDs {
	p := make(ParentMessageIDs)
	return p
}

// AddStrong adds a strong parent to the map.
func (p ParentMessageIDs) AddStrong(messageID MessageID) ParentMessageIDs {
	if _, exists := p[StrongParentType]; !exists {
		p[StrongParentType] = NewMessageIDs()
	}
	return p.Add(StrongParentType, messageID)
}

// Add adds a parent to the map.
func (p ParentMessageIDs) Add(parentType ParentsType, messageID MessageID) ParentMessageIDs {
	if _, exists := p[parentType]; !exists {
		p[parentType] = NewMessageIDs()
	}
	p[parentType].Add(messageID)
	return p
}

// AddAll adds a collection of parents to the map.
func (p ParentMessageIDs) AddAll(parentType ParentsType, messageIDs MessageIDs) ParentMessageIDs {
	if _, exists := p[parentType]; !exists {
		p[parentType] = NewMessageIDs()
	}
	p[parentType].AddAll(messageIDs)
	return p
}

// Clone returns a copy of map.
func (p ParentMessageIDs) Clone() ParentMessageIDs {
	pCloned := NewParentMessageIDs()
	for parentType, messageIDs := range p {
		if _, exists := p[parentType]; !exists {
			p[parentType] = NewMessageIDs()
		}
		pCloned.AddAll(parentType, messageIDs)
	}
	return pCloned
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	messageMetadataInner `serix:"0"`
}

type messageMetadataInner struct {
	MessageID           MessageID
	ReceivedTime        time.Time                 `serix:"1"`
	SolidificationTime  time.Time                 `serix:"2"`
	Solid               bool                      `serix:"3"`
	StructureDetails    *markers.StructureDetails `serix:"4,optional"`
	AddedBranchIDs      ledgerstate.BranchIDs     `serix:"5"`
	SubtractedBranchIDs ledgerstate.BranchIDs     `serix:"6"`
	Scheduled           bool                      `serix:"7"`
	ScheduledTime       time.Time                 `serix:"8"`
	Booked              bool                      `serix:"9"`
	BookedTime          time.Time                 `serix:"10"`
	ObjectivelyInvalid  bool                      `serix:"11"`
	GradeOfFinality     gof.GradeOfFinality       `serix:"12"`
	GradeOfFinalityTime time.Time                 `serix:"13"`
	DiscardedTime       time.Time                 `serix:"14"`
	QueuedTime          time.Time                 `serix:"15"`
	SubjectivelyInvalid bool                      `serix:"16"`

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

	objectstorage.StorableObjectFlags
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID MessageID) *MessageMetadata {
	return &MessageMetadata{
		messageMetadataInner{
			MessageID:           messageID,
			ReceivedTime:        clock.SyncedTime(),
			AddedBranchIDs:      ledgerstate.NewBranchIDs(),
			SubtractedBranchIDs: ledgerstate.NewBranchIDs(),
		},
	}
}

// FromObjectStorage creates an MessageMetadata from sequences of key and bytes.
func (m *MessageMetadata) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := m.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = fmt.Errorf("failed to parse message metadata from object storage: %w", err)
	}
	return result, err
}

// FromBytes unmarshals the given bytes into a MessageMetadata.
func (m *MessageMetadata) FromBytes(data []byte) (result *MessageMetadata, err error) {
	msgMetadata := new(MessageMetadata)
	if m != nil {
		msgMetadata = m
	}
	messageID := new(MessageID)
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, messageID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse MessageMetadata.MessageID: %w", err)
		return msgMetadata, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], msgMetadata, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse MessageMetadata: %w", err)
		return msgMetadata, err
	}
	msgMetadata.messageMetadataInner.MessageID = *messageID
	return msgMetadata, err
}

// ID returns the MessageID of the Message that this MessageMetadata object belongs to.
func (m *MessageMetadata) ID() MessageID {
	return m.messageMetadataInner.MessageID
}

// ReceivedTime returns the time when the message was received.
func (m *MessageMetadata) ReceivedTime() time.Time {
	return m.messageMetadataInner.ReceivedTime
}

// IsSolid returns true if the message represented by this metadata is solid. False otherwise.
func (m *MessageMetadata) IsSolid() (result bool) {
	m.solidMutex.RLock()
	defer m.solidMutex.RUnlock()
	result = m.messageMetadataInner.Solid

	return
}

// SetSolid sets the message associated with this metadata as solid.
// It returns true if the solid status is modified. False otherwise.
func (m *MessageMetadata) SetSolid(solid bool) (modified bool) {
	m.solidMutex.RLock()
	if m.messageMetadataInner.Solid != solid {
		m.solidMutex.RUnlock()

		m.solidMutex.Lock()
		if m.messageMetadataInner.Solid != solid {
			m.messageMetadataInner.Solid = solid
			if solid {
				m.solidificationTimeMutex.Lock()
				m.messageMetadataInner.SolidificationTime = clock.SyncedTime()
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

	return m.messageMetadataInner.SolidificationTime
}

// SetStructureDetails sets the structureDetails of the message.
func (m *MessageMetadata) SetStructureDetails(structureDetails *markers.StructureDetails) (modified bool) {
	m.structureDetailsMutex.Lock()
	defer m.structureDetailsMutex.Unlock()

	if m.messageMetadataInner.StructureDetails != nil {
		return false
	}

	m.messageMetadataInner.StructureDetails = structureDetails

	m.SetModified()
	return true
}

// StructureDetails returns the structureDetails of the message.
func (m *MessageMetadata) StructureDetails() *markers.StructureDetails {
	m.structureDetailsMutex.RLock()
	defer m.structureDetailsMutex.RUnlock()

	return m.messageMetadataInner.StructureDetails
}

// SetAddedBranchIDs sets the BranchIDs of the added Branches.
func (m *MessageMetadata) SetAddedBranchIDs(addedBranchIDs ledgerstate.BranchIDs) (modified bool) {
	m.addedBranchIDsMutex.Lock()
	defer m.addedBranchIDsMutex.Unlock()

	if m.messageMetadataInner.AddedBranchIDs.Equals(addedBranchIDs) {
		return false
	}

	m.messageMetadataInner.AddedBranchIDs = addedBranchIDs.Clone()
	m.SetModified(true)
	modified = true

	return
}

// AddBranchID sets the BranchIDs of the added Branches.
func (m *MessageMetadata) AddBranchID(branchID ledgerstate.BranchID) (modified bool) {
	m.addedBranchIDsMutex.Lock()
	defer m.addedBranchIDsMutex.Unlock()

	if m.messageMetadataInner.AddedBranchIDs.Contains(branchID) {
		return
	}

	m.messageMetadataInner.AddedBranchIDs.Add(branchID)
	m.SetModified(true)
	return true
}

// AddedBranchIDs returns the BranchIDs of the added Branches of the Message.
func (m *MessageMetadata) AddedBranchIDs() ledgerstate.BranchIDs {
	m.addedBranchIDsMutex.RLock()
	defer m.addedBranchIDsMutex.RUnlock()

	return m.messageMetadataInner.AddedBranchIDs.Clone()
}

// SetSubtractedBranchIDs sets the BranchIDs of the subtracted Branches.
func (m *MessageMetadata) SetSubtractedBranchIDs(subtractedBranchIDs ledgerstate.BranchIDs) (modified bool) {
	m.subtractedBranchIDsMutex.Lock()
	defer m.subtractedBranchIDsMutex.Unlock()

	if m.messageMetadataInner.SubtractedBranchIDs.Equals(subtractedBranchIDs) {
		return false
	}

	m.messageMetadataInner.SubtractedBranchIDs = subtractedBranchIDs.Clone()
	m.SetModified(true)
	modified = true

	return
}

// SubtractedBranchIDs returns the BranchIDs of the subtracted Branches of the Message.
func (m *MessageMetadata) SubtractedBranchIDs() ledgerstate.BranchIDs {
	m.subtractedBranchIDsMutex.RLock()
	defer m.subtractedBranchIDsMutex.RUnlock()

	return m.messageMetadataInner.SubtractedBranchIDs.Clone()
}

// SetScheduled sets the message associated with this metadata as scheduled.
// It returns true if the scheduled status is modified. False otherwise.
func (m *MessageMetadata) SetScheduled(scheduled bool) (modified bool) {
	m.scheduledMutex.Lock()
	defer m.scheduledMutex.Unlock()
	m.scheduledTimeMutex.Lock()
	defer m.scheduledTimeMutex.Unlock()

	if m.messageMetadataInner.Scheduled == scheduled {
		return false
	}

	m.messageMetadataInner.Scheduled = scheduled
	m.messageMetadataInner.ScheduledTime = clock.SyncedTime()
	m.SetModified()
	modified = true

	return
}

// Scheduled returns true if the message represented by this metadata was scheduled. False otherwise.
func (m *MessageMetadata) Scheduled() (result bool) {
	m.scheduledMutex.RLock()
	defer m.scheduledMutex.RUnlock()

	return m.messageMetadataInner.Scheduled
}

// ScheduledTime returns the time when the message represented by this metadata was scheduled.
func (m *MessageMetadata) ScheduledTime() time.Time {
	m.scheduledTimeMutex.RLock()
	defer m.scheduledTimeMutex.RUnlock()

	return m.messageMetadataInner.ScheduledTime
}

// SetDiscardedTime add the discarded time of a message to the metadata.
func (m *MessageMetadata) SetDiscardedTime(discardedTime time.Time) {
	m.discardedTimeMutex.Lock()
	defer m.discardedTimeMutex.Unlock()

	m.messageMetadataInner.DiscardedTime = discardedTime
}

// DiscardedTime returns when the message was discarded.
func (m *MessageMetadata) DiscardedTime() time.Time {
	m.discardedTimeMutex.RLock()
	defer m.discardedTimeMutex.RUnlock()

	return m.messageMetadataInner.DiscardedTime
}

// QueuedTime returns the time a message entered the scheduling queue.
func (m *MessageMetadata) QueuedTime() time.Time {
	m.queuedTimeMutex.RLock()
	defer m.queuedTimeMutex.RUnlock()

	return m.messageMetadataInner.QueuedTime
}

// SetQueuedTime records the time the message entered the scheduler queue.
func (m *MessageMetadata) SetQueuedTime(queuedTime time.Time) {
	m.queuedTimeMutex.Lock()
	defer m.queuedTimeMutex.Unlock()

	m.messageMetadataInner.QueuedTime = queuedTime
}

// SetBooked sets the message associated with this metadata as booked.
// It returns true if the booked status is modified. False otherwise.
func (m *MessageMetadata) SetBooked(booked bool) (modified bool) {
	m.bookedMutex.Lock()
	defer m.bookedMutex.Unlock()
	m.bookedTimeMutex.Lock()
	defer m.bookedTimeMutex.Unlock()

	if m.messageMetadataInner.Booked == booked {
		return false
	}

	m.messageMetadataInner.Booked = booked
	m.messageMetadataInner.BookedTime = clock.SyncedTime()
	m.SetModified()
	modified = true

	return
}

// IsBooked returns true if the message represented by this metadata is booked. False otherwise.
func (m *MessageMetadata) IsBooked() (result bool) {
	m.bookedMutex.RLock()
	defer m.bookedMutex.RUnlock()
	result = m.messageMetadataInner.Booked

	return
}

// BookedTime returns the time when the message represented by this metadata was booked.
func (m *MessageMetadata) BookedTime() time.Time {
	m.bookedTimeMutex.RLock()
	defer m.bookedTimeMutex.RUnlock()

	return m.messageMetadataInner.BookedTime
}

// IsObjectivelyInvalid returns true if the message represented by this metadata is objectively invalid.
func (m *MessageMetadata) IsObjectivelyInvalid() (result bool) {
	m.invalidMutex.RLock()
	defer m.invalidMutex.RUnlock()
	result = m.messageMetadataInner.ObjectivelyInvalid

	return
}

// SetObjectivelyInvalid sets the message associated with this metadata as objectively invalid - it returns true if the
// status was changed.
func (m *MessageMetadata) SetObjectivelyInvalid(invalid bool) (modified bool) {
	m.invalidMutex.Lock()
	defer m.invalidMutex.Unlock()

	if m.messageMetadataInner.ObjectivelyInvalid == invalid {
		return false
	}

	m.messageMetadataInner.ObjectivelyInvalid = invalid
	m.SetModified()
	modified = true

	return
}

// IsSubjectivelyInvalid returns true if the message represented by this metadata is subjectively invalid.
func (m *MessageMetadata) IsSubjectivelyInvalid() (result bool) {
	m.invalidMutex.RLock()
	defer m.invalidMutex.RUnlock()
	result = m.messageMetadataInner.SubjectivelyInvalid

	return
}

// SetSubjectivelyInvalid sets the message associated with this metadata as subjectively invalid - it returns true if
// the status was changed.
func (m *MessageMetadata) SetSubjectivelyInvalid(invalid bool) (modified bool) {
	m.invalidMutex.Lock()
	defer m.invalidMutex.Unlock()

	if m.messageMetadataInner.SubjectivelyInvalid == invalid {
		return false
	}

	m.messageMetadataInner.SubjectivelyInvalid = invalid
	m.SetModified()
	modified = true

	return
}

// SetGradeOfFinality sets the grade of finality associated with this metadata.
// It returns true if the grade of finality is modified. False otherwise.
func (m *MessageMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	m.gradeOfFinalityMutex.Lock()
	defer m.gradeOfFinalityMutex.Unlock()

	if m.messageMetadataInner.GradeOfFinality == gradeOfFinality {
		return false
	}

	m.messageMetadataInner.GradeOfFinality = gradeOfFinality
	m.messageMetadataInner.GradeOfFinalityTime = clock.SyncedTime()
	m.SetModified()
	modified = true

	return
}

// GradeOfFinality returns the grade of finality.
func (m *MessageMetadata) GradeOfFinality() (result gof.GradeOfFinality) {
	m.gradeOfFinalityMutex.RLock()
	defer m.gradeOfFinalityMutex.RUnlock()

	return m.messageMetadataInner.GradeOfFinality
}

// GradeOfFinalityTime returns the time the grade of finality was set.
func (m *MessageMetadata) GradeOfFinalityTime() time.Time {
	m.gradeOfFinalityMutex.RLock()
	defer m.gradeOfFinalityMutex.RUnlock()

	return m.messageMetadataInner.GradeOfFinalityTime
}

// Bytes returns a marshaled Version of the whole MessageMetadata object.
func (m *MessageMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MessageMetadata) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m.MessageID, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the MessageMetadata into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (m *MessageMetadata) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable Version of the MessageMetadata.
func (m *MessageMetadata) String() string {
	return stringify.Struct("MessageMetadata",
		stringify.StructField("ID", m.MessageID),
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
	// ErrBlockTypeIsUnknown is triggered when the block type is unknown.
	ErrBlockTypeIsUnknown = errors.Errorf("block types must range from %d-%d", 1, LastValidBlockType)
	// ErrConflictingReferenceAcrossBlocks is triggered if there conflicting references across blocks.
	ErrConflictingReferenceAcrossBlocks = errors.New("different blocks have conflicting references")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
