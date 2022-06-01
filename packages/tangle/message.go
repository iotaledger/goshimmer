package tangle

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
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
	MessageIDLength = types.IdentifierLength

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
type MessageID struct {
	types.Identifier `serix:"0"`
}

// EmptyMessageID is an empty id.
var EmptyMessageID MessageID

// NewMessageID returns a new MessageID for the given data.
func NewMessageID(bytes [32]byte) (new MessageID) {
	return MessageID{Identifier: bytes}
}

// Length returns the byte length of a serialized TransactionID.
func (m MessageID) Length() int {
	return types.IdentifierLength
}

// String returns a human-readable version of the MessageID.
func (m MessageID) String() (humanReadable string) {
	return "MessageID(" + m.Alias() + ")"
}

// CompareTo does a lexicographical comparison to another messageID.
// Returns 0 if equal, -1 if smaller, or 1 if larger than other.
// Passing nil as other will result in a panic.
func (m MessageID) CompareTo(other MessageID) int {
	return bytes.Compare(m.Bytes(), other.Bytes())
}

func MessageIDFromContext(ctx context.Context) MessageID {
	messageID, ok := ctx.Value("messageID").(MessageID)
	if !ok {
		return EmptyMessageID
	}
	return messageID
}

func MessageIDToContext(ctx context.Context, messageID MessageID) context.Context {
	return context.WithValue(ctx, "messageID", messageID)
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
	model.Storable[MessageID, MessageModel] `serix:"0"`
}
type MessageModel struct {
	// core properties (get sent over the wire)
	Version         uint8             `serix:"0"`
	Parents         ParentMessageIDs  `serix:"1"`
	IssuerPublicKey ed25519.PublicKey `serix:"2"`
	IssuingTime     time.Time         `serix:"3"`
	SequenceNumber  uint64            `serix:"4"`
	Payload         payload.Payload   `serix:"5,optional"`
	Nonce           uint64            `serix:"6"`
	Signature       ed25519.Signature `serix:"7"`
}

// NewMessage creates a new message with the details provided by the issuer.
func NewMessage(references ParentMessageIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, msgPayload payload.Payload, nonce uint64, signature ed25519.Signature, versionOpt ...uint8) *Message {
	version := MessageVersion
	if len(versionOpt) == 1 {
		version = versionOpt[0]
	}
	msg := &Message{model.NewStorable[MessageID, MessageModel](MessageModel{
		Version:         version,
		Parents:         references,
		IssuerPublicKey: issuerPublicKey,
		IssuingTime:     issuingTime,
		SequenceNumber:  sequenceNumber,
		Payload:         msgPayload,
		Nonce:           nonce,
		Signature:       signature,
	})}

	return msg
}

// NewMessageWithValidation creates a new message while performing ths following syntactical checks:
// 1. A Strong Parents Block must exist.
// 2. Parents Block types cannot repeat.
// 3. Parent count per block 1 <= x <= 8.
// 4. Parents unique within block.
// 5. Parents lexicographically sorted within block.
// 7. Blocks should be ordered by type in ascending order.
// 6. A Parent(s) repetition is only allowed when it occurs across Strong and Like parents.
func NewMessageWithValidation(references ParentMessageIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, msgPayload payload.Payload, nonce uint64, signature ed25519.Signature, version ...uint8) (result *Message, err error) {
	msg := NewMessage(references, issuingTime, issuerPublicKey, sequenceNumber, msgPayload, nonce, signature, version...)

	if _, err = msg.Bytes(); err != nil {
		return nil, errors.Errorf("failed to create message: %w", err)
	}
	return msg, nil
}

// // FromBytes unmarshals a Transaction from a sequence of bytes.
// func (m *Message) FromBytes(data []byte) (err error) {
// 	consumedBytes, err := serix.DefaultAPI.Decode(context.Background(), data, m, serix.WithValidation())
// 	if err != nil {
// 		return errors.Errorf("failed to parse Message: %w", err)
// 	}
//
// 	if len(data) != consumedBytes {
// 		return errors.Errorf("consumed bytes %d not equal total bytes %d: %w", consumedBytes, len(data), cerrors.ErrParseBytesFailed)
// 	}
//
// 	// TODO: this seems a bit out of place here.
// 	// msgPayload := msg.Payload()
// 	// if msgPayload != nil && msgPayload.Type() == devnetvm.TransactionType {
// 	// 	tx := msgPayload.(*devnetvm.Transaction)
// 	//
// 	// 	devnetvm.SetOutputID(tx.Essence(), tx.ID())
// 	// }
//
// 	return nil
// }

// VerifySignature verifies the Signature of the message.
func (m *Message) VerifySignature() (valid bool, err error) {
	msgBytes, err := m.Bytes()
	if err != nil {
		return false, errors.Errorf("failed to create message bytes: %w", err)
	}
	signature := m.Signature()

	contentLength := len(msgBytes) - len(signature)
	content := msgBytes[:contentLength]

	return m.M.IssuerPublicKey.VerifySignature(content, signature), nil
}

// IDBytes implements Element interface in scheduler NodeQueue that returns the MessageID of the message in bytes.
func (m *Message) IDBytes() []byte {
	return m.ID().Bytes()
}

// Version returns the message Version.
func (m *Message) Version() uint8 {
	return m.M.Version
}

// ParentsByType returns a slice of all parents of the desired type.
func (m *Message) ParentsByType(parentType ParentsType) MessageIDs {
	if parents, ok := m.M.Parents[parentType]; ok {
		return parents.Clone()
	}
	return NewMessageIDs()
}

// ForEachParent executes a consumer func for each parent.
func (m *Message) ForEachParent(consumer func(parent Parent)) {
	for parentType, parents := range m.M.Parents {
		for parentID := range parents {
			consumer(Parent{
				Type: parentType,
				ID:   parentID,
			})
		}
	}
}

func (m *Message) Parents() (parents []MessageID) {
	m.ForEachParent(func(parent Parent) {
		parents = append(parents, parent.ID)
	})
	return
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
	return m.M.IssuerPublicKey
}

// IssuingTime returns the time when this message was created.
func (m *Message) IssuingTime() time.Time {
	return m.M.IssuingTime
}

// SequenceNumber returns the sequence number of this message.
func (m *Message) SequenceNumber() uint64 {
	return m.M.SequenceNumber
}

// Payload returns the Payload of the message.
func (m *Message) Payload() payload.Payload {
	return m.M.Payload
}

// Nonce returns the Nonce of the message.
func (m *Message) Nonce() uint64 {
	return m.M.Nonce
}

// Signature returns the Signature of the message.
func (m *Message) Signature() ed25519.Signature {
	return m.M.Signature
}

// DetermineID calculates and sets the message's MessageID.
func (m *Message) DetermineID() (err error) {
	b, err := m.Bytes()
	if err != nil {
		return errors.Errorf("failed to determine message ID: %w", err)
	}

	m.SetID(MessageID{Identifier: blake2b.Sum256(b)})
	return nil
}

// Size returns the message size in bytes.
func (m *Message) Size() int {
	return len(lo.PanicOnErr(m.Bytes()))
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
	model.Storable[MessageID, messageMetadataModel] `serix:"0"`
}

type messageMetadataModel struct {
	ReceivedTime        time.Time                 `serix:"1"`
	SolidificationTime  time.Time                 `serix:"2"`
	Solid               bool                      `serix:"3"`
	StructureDetails    *markers.StructureDetails `serix:"4,optional"`
	AddedBranchIDs      utxo.TransactionIDs       `serix:"5"`
	SubtractedBranchIDs utxo.TransactionIDs       `serix:"6"`
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
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID MessageID) *MessageMetadata {
	meta := &MessageMetadata{model.NewStorable[MessageID](messageMetadataModel{
		ReceivedTime:        clock.SyncedTime(),
		AddedBranchIDs:      utxo.NewTransactionIDs(),
		SubtractedBranchIDs: utxo.NewTransactionIDs(),
	})}
	meta.SetID(messageID)

	return meta
}

// ReceivedTime returns the time when the message was received.
func (m *MessageMetadata) ReceivedTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.ReceivedTime
}

// IsSolid returns true if the message represented by this metadata is solid. False otherwise.
func (m *MessageMetadata) IsSolid() (result bool) {
	m.RLock()
	defer m.RUnlock()

	return m.M.Solid
}

// SetSolid sets the message associated with this metadata as solid.
// It returns true if the solid status is modified. False otherwise.
func (m *MessageMetadata) SetSolid(solid bool) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.Solid == solid {
		return false
	}

	m.M.SolidificationTime = clock.SyncedTime()
	m.M.Solid = solid
	m.SetModified()
	return true
}

// SolidificationTime returns the time when the message was marked to be solid.
func (m *MessageMetadata) SolidificationTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.SolidificationTime
}

// SetStructureDetails sets the structureDetails of the message.
func (m *MessageMetadata) SetStructureDetails(structureDetails *markers.StructureDetails) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.StructureDetails != nil {
		return false
	}

	m.M.StructureDetails = structureDetails

	m.SetModified()
	return true
}

// StructureDetails returns the structureDetails of the message.
func (m *MessageMetadata) StructureDetails() *markers.StructureDetails {
	m.RLock()
	defer m.RUnlock()

	return m.M.StructureDetails
}

// SetAddedBranchIDs sets the BranchIDs of the added Branches.
func (m *MessageMetadata) SetAddedBranchIDs(addedBranchIDs utxo.TransactionIDs) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.AddedBranchIDs.Equal(addedBranchIDs) {
		return false
	}

	m.M.AddedBranchIDs = addedBranchIDs.Clone()
	m.SetModified()
	return true
}

// AddBranchID sets the BranchIDs of the added Branches.
func (m *MessageMetadata) AddBranchID(branchID utxo.TransactionID) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.AddedBranchIDs.Has(branchID) {
		return
	}

	m.M.AddedBranchIDs.Add(branchID)
	m.SetModified()
	return true
}

// AddedBranchIDs returns the BranchIDs of the added Branches of the Message.
func (m *MessageMetadata) AddedBranchIDs() utxo.TransactionIDs {
	m.RLock()
	defer m.RUnlock()

	return m.M.AddedBranchIDs.Clone()
}

// SetSubtractedBranchIDs sets the BranchIDs of the subtracted Branches.
func (m *MessageMetadata) SetSubtractedBranchIDs(subtractedBranchIDs utxo.TransactionIDs) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.SubtractedBranchIDs.Equal(subtractedBranchIDs) {
		return false
	}

	m.M.SubtractedBranchIDs = subtractedBranchIDs.Clone()
	m.SetModified()
	return true
}

// SubtractedBranchIDs returns the BranchIDs of the subtracted Branches of the Message.
func (m *MessageMetadata) SubtractedBranchIDs() utxo.TransactionIDs {
	m.RLock()
	defer m.RUnlock()

	return m.M.SubtractedBranchIDs.Clone()
}

// SetScheduled sets the message associated with this metadata as scheduled.
// It returns true if the scheduled status is modified. False otherwise.
func (m *MessageMetadata) SetScheduled(scheduled bool) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.Scheduled == scheduled {
		return false
	}

	m.M.Scheduled = scheduled
	m.M.ScheduledTime = clock.SyncedTime()
	m.SetModified()
	return true
}

// Scheduled returns true if the message represented by this metadata was scheduled. False otherwise.
func (m *MessageMetadata) Scheduled() bool {
	m.RLock()
	defer m.RUnlock()

	return m.M.Scheduled
}

// ScheduledTime returns the time when the message represented by this metadata was scheduled.
func (m *MessageMetadata) ScheduledTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.ScheduledTime
}

// SetDiscardedTime add the discarded time of a message to the metadata.
func (m *MessageMetadata) SetDiscardedTime(discardedTime time.Time) {
	m.Lock()
	defer m.Unlock()

	m.M.DiscardedTime = discardedTime
	m.SetModified()
}

// DiscardedTime returns when the message was discarded.
func (m *MessageMetadata) DiscardedTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.DiscardedTime
}

// QueuedTime returns the time a message entered the scheduling queue.
func (m *MessageMetadata) QueuedTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.QueuedTime
}

// SetQueuedTime records the time the message entered the scheduler queue.
func (m *MessageMetadata) SetQueuedTime(queuedTime time.Time) {
	m.Lock()
	defer m.Unlock()

	m.M.QueuedTime = queuedTime
	m.SetModified()
}

// SetBooked sets the message associated with this metadata as booked.
// It returns true if the booked status is modified. False otherwise.
func (m *MessageMetadata) SetBooked(booked bool) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.Booked == booked {
		return false
	}

	m.M.Booked = booked
	m.M.BookedTime = clock.SyncedTime()
	m.SetModified()
	return true
}

// IsBooked returns true if the message represented by this metadata is booked. False otherwise.
func (m *MessageMetadata) IsBooked() (result bool) {
	m.RLock()
	defer m.RUnlock()

	return m.M.Booked
}

// BookedTime returns the time when the message represented by this metadata was booked.
func (m *MessageMetadata) BookedTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.BookedTime
}

// IsObjectivelyInvalid returns true if the message represented by this metadata is objectively invalid.
func (m *MessageMetadata) IsObjectivelyInvalid() (result bool) {
	m.RLock()
	defer m.RUnlock()

	return m.M.ObjectivelyInvalid
}

// SetObjectivelyInvalid sets the message associated with this metadata as objectively invalid - it returns true if the
// status was changed.
func (m *MessageMetadata) SetObjectivelyInvalid(invalid bool) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.ObjectivelyInvalid == invalid {
		return false
	}

	m.M.ObjectivelyInvalid = invalid
	m.SetModified()
	return true
}

// IsSubjectivelyInvalid returns true if the message represented by this metadata is subjectively invalid.
func (m *MessageMetadata) IsSubjectivelyInvalid() (result bool) {
	m.RLock()
	defer m.RUnlock()

	return m.M.SubjectivelyInvalid
}

// SetSubjectivelyInvalid sets the message associated with this metadata as subjectively invalid - it returns true if
// the status was changed.
func (m *MessageMetadata) SetSubjectivelyInvalid(invalid bool) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.SubjectivelyInvalid == invalid {
		return false
	}

	m.M.SubjectivelyInvalid = invalid
	m.SetModified()
	return true
}

// SetGradeOfFinality sets the grade of finality associated with this metadata.
// It returns true if the grade of finality is modified. False otherwise.
func (m *MessageMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	m.Lock()
	defer m.Unlock()

	if m.M.GradeOfFinality == gradeOfFinality {
		return false
	}

	m.M.GradeOfFinality = gradeOfFinality
	m.M.GradeOfFinalityTime = clock.SyncedTime()
	m.SetModified()
	return true
}

// GradeOfFinality returns the grade of finality.
func (m *MessageMetadata) GradeOfFinality() (result gof.GradeOfFinality) {
	m.RLock()
	defer m.RUnlock()

	return m.M.GradeOfFinality
}

// GradeOfFinalityTime returns the time the grade of finality was set.
func (m *MessageMetadata) GradeOfFinalityTime() time.Time {
	m.RLock()
	defer m.RUnlock()

	return m.M.GradeOfFinalityTime
}

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
