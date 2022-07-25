package tangle

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

func init() {
	blockIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsCount,
		Max:            MaxParentsCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
	}
	err := serix.DefaultAPI.RegisterTypeSettings(BlockIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(blockIDsArrayRules))
	if err != nil {
		panic(fmt.Errorf("error registering BlockIDs type settings: %w", err))
	}
	parentsBlockIDsArrayRules := &serix.ArrayRules{
		Min:            MinParentsBlocksCount,
		Max:            MaxParentsBlocksCount,
		ValidationMode: serializer.ArrayValidationModeNoDuplicates,
		UniquenessSliceFunc: func(next []byte) []byte {
			// return first byte which indicates the parent type
			return next[:1]
		},
	}
	err = serix.DefaultAPI.RegisterTypeSettings(ParentBlockIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsByte).WithArrayRules(parentsBlockIDsArrayRules))
	if err != nil {
		panic(fmt.Errorf("error registering ParentBlockIDs type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterValidators(ParentBlockIDs{}, validateParentBlockIDsBytes, validateParentBlockIDs)

	if err != nil {
		panic(fmt.Errorf("error registering ParentBlockIDs validators: %w", err))
	}
}

func validateParentBlockIDs(_ context.Context, parents ParentBlockIDs) (err error) {
	// Validate strong parent block
	if strongParents, strongParentsExist := parents[StrongParentType]; len(parents) == 0 || !strongParentsExist ||
		len(strongParents) < MinStrongParentsCount {
		return ErrNoStrongParents
	}
	for parentsType := range parents {
		if parentsType > LastValidBlockType {
			return ErrBlockTypeIsUnknown
		}
	}
	if areReferencesConflictingAcrossBlocks(parents) {
		return ErrConflictingReferenceAcrossBlocks
	}
	return nil
}

// validate blocksIDs are unique across blocks
// there may be repetition across strong and like parents.
func areReferencesConflictingAcrossBlocks(parentsBlocks ParentBlockIDs) bool {
	for blockID := range parentsBlocks[WeakParentType] {
		if _, exists := parentsBlocks[StrongParentType][blockID]; exists {
			return true
		}

		if _, exists := parentsBlocks[ShallowLikeParentType][blockID]; exists {
			return true
		}
	}

	return false
}

func validateParentBlockIDsBytes(_ context.Context, _ []byte) (err error) {
	return
}

const (
	// BlockVersion defines the Version of the block structure.
	BlockVersion uint8 = 1

	// MaxBlockSize defines the maximum size of a block.
	MaxBlockSize = 64 * 1024

	// BlockIDLength defines the length of an BlockID.
	BlockIDLength = types.IdentifierLength + 8

	// MinParentsCount defines the minimum number of parents each parents block must have.
	MinParentsCount = 1

	// MaxParentsCount defines the maximum number of parents each parents block must have.
	MaxParentsCount = 8

	// MinParentsBlocksCount defines the minimum number of parents each parents block must have.
	MinParentsBlocksCount = 1

	// MaxParentsBlocksCount defines the maximum number of parents each parents block must have.
	MaxParentsBlocksCount = 4

	// MinStrongParentsCount defines the minimum number of strong parents a block must have.
	MinStrongParentsCount = 1
)

// region BlockID ////////////////////////////////////////////////////////////////////////////////////////////////////

// BlockID identifies a block via its BLAKE2b-256 hash of its bytes.
type BlockID struct {
	Identifier types.Identifier `serix:"0"`
	EpochIndex epoch.Index      `serix:"1"`
}

// EmptyBlockID is an empty id.
var EmptyBlockID BlockID

// NewBlockID returns a new BlockID for the given data.
func NewBlockID(identifier [32]byte, epochIndex epoch.Index) (new BlockID) {
	return BlockID{
		Identifier: identifier,
		EpochIndex: epochIndex,
	}
}

// FromBytes deserializes a BlockID from a byte slice.
func (b *BlockID) FromBytes(serialized []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), serialized, b, serix.WithValidation())
}

// FromBase58 un-serializes a BlockID from a base58 encoded string.
func (b *BlockID) FromBase58(base58EncodedString string) (err error) {
	s := strings.Split(base58EncodedString, ":")
	decodedBytes, err := base58.Decode(s[0])
	if err != nil {
		return errors.Errorf("could not decode base58 encoded BlockID.Identifier: %w", err)
	}
	epochIndex, err := strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return errors.Errorf("could not decode BlockID.EpochIndex from string: %w", err)
	}

	if _, err = serix.DefaultAPI.Decode(context.Background(), decodedBytes, &b.Identifier, serix.WithValidation()); err != nil {
		return errors.Errorf("failed to decode BlockID: %w", err)
	}
	b.EpochIndex = epoch.Index(epochIndex)

	return nil
}

// FromRandomness generates a random BlockID.
func (b *BlockID) FromRandomness(optionalEpoch ...epoch.Index) (err error) {
	if err = b.Identifier.FromRandomness(); err != nil {
		return errors.Errorf("could not create Identifier from randomness: %w", err)
	}

	if len(optionalEpoch) >= 1 {
		b.EpochIndex = optionalEpoch[0]
	}

	return nil
}

// Alias returns the human-readable alias of the BlockID (or the base58 encoded bytes if no alias was set).
func (b BlockID) Alias() (alias string) {
	_BlockIDAliasesMutex.RLock()
	defer _BlockIDAliasesMutex.RUnlock()

	if existingAlias, exists := _BlockIDAliases[b]; exists {
		return fmt.Sprintf("%s", existingAlias)
	}

	return fmt.Sprintf("%s, %d", b.Identifier, int(b.EpochIndex))
}

// RegisterAlias allows to register a human-readable alias for the BlockID which will be used as a replacement for the
// String method.
func (b BlockID) RegisterAlias(alias string) {
	_BlockIDAliasesMutex.Lock()
	defer _BlockIDAliasesMutex.Unlock()

	_BlockIDAliases[b] = alias
}

// UnregisterAlias allows to unregister a previously registered alias.
func (b BlockID) UnregisterAlias() {
	_BlockIDAliasesMutex.Lock()
	defer _BlockIDAliasesMutex.Unlock()

	delete(_BlockIDAliases, b)
}

// Base58 returns a base58 encoded version of the BlockID.
func (b BlockID) Base58() (base58Encoded string) {
	return fmt.Sprintf("%s:%s", base58.Encode(b.Identifier[:]), strconv.FormatInt(int64(b.EpochIndex), 10))
}

// Length returns the byte length of a serialized BlockID.
func (b BlockID) Length() int {
	return BlockIDLength
}

// Bytes returns a serialized version of the BlockID.
func (b BlockID) Bytes() (serialized []byte) {
	return lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation()))
}

// String returns a human-readable version of the BlockID.
func (b BlockID) String() (humanReadable string) {
	return "BlockID(" + b.Alias() + ")"
}

// CompareTo does a lexicographical comparison to another blockID.
// Returns 0 if equal, -1 if smaller, or 1 if larger than other.
// Passing nil as other will result in a panic.
func (b BlockID) CompareTo(other BlockID) int {
	return bytes.Compare(b.Bytes(), other.Bytes())
}

// BlockIDFromContext returns the BlockID from the given context.
func BlockIDFromContext(ctx context.Context) BlockID {
	blockID, ok := ctx.Value("blockID").(BlockID)
	if !ok {
		return EmptyBlockID
	}
	return blockID
}

// BlockIDToContext adds the BlockID to the given context.
func BlockIDToContext(ctx context.Context, blockID BlockID) context.Context {
	return context.WithValue(ctx, "blockID", blockID)
}

var (
	// _BlockIDAliases contains a dictionary of BlockIDs associated to their human-readable alias.
	_BlockIDAliases = make(map[BlockID]string)

	// _BlockIDAliasesMutex is the mutex that is used to synchronize access to the previous map.
	_BlockIDAliasesMutex = sync.RWMutex{}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockIDs ///////////////////////////////////////////////////////////////////////////////////////////////////

// BlockIDs is a set of BlockIDs where every BlockID is stored only once.
type BlockIDs map[BlockID]types.Empty

// NewBlockIDs construct a new BlockID collection from the optional BlockIDs.
func NewBlockIDs(blkIDs ...BlockID) BlockIDs {
	m := make(BlockIDs)
	for _, blkID := range blkIDs {
		m[blkID] = types.Void
	}

	return m
}

// Slice converts the set of BlockIDs into a slice of BlockIDs.
func (m BlockIDs) Slice() []BlockID {
	ids := make([]BlockID, 0)
	for key := range m {
		ids = append(ids, key)
	}
	return ids
}

// Clone creates a copy of the BlockIDs.
func (m BlockIDs) Clone() (clonedBlockIDs BlockIDs) {
	clonedBlockIDs = make(BlockIDs)
	for key, value := range m {
		clonedBlockIDs[key] = value
	}
	return
}

// Add adds a BlockID to the collection and returns the collection to enable chaining.
func (m BlockIDs) Add(blockID BlockID) BlockIDs {
	m[blockID] = types.Void

	return m
}

// AddAll adds all BlockIDs to the collection and returns the collection to enable chaining.
func (m BlockIDs) AddAll(blockIDs BlockIDs) BlockIDs {
	for blockID := range blockIDs {
		m.Add(blockID)
	}

	return m
}

// Empty checks if BlockIDs is empty.
func (m BlockIDs) Empty() (empty bool) {
	return len(m) == 0
}

// Contains checks if the given target BlockID is part of the BlockIDs.
func (m BlockIDs) Contains(target BlockID) (contains bool) {
	_, contains = m[target]
	return
}

// Subtract removes all other from the collection and returns the collection to enable chaining.
func (m BlockIDs) Subtract(other BlockIDs) BlockIDs {
	for blockID := range other {
		delete(m, blockID)
	}

	return m
}

// First returns the first element in BlockIDs (not ordered). This method only makes sense if there is exactly one
// element in the collection.
func (m BlockIDs) First() BlockID {
	for blockID := range m {
		return blockID
	}
	return EmptyBlockID
}

// Base58 returns a string slice of base58 BlockID.
func (m BlockIDs) Base58() (result []string) {
	result = make([]string, 0, len(m))
	for id := range m {
		result = append(result, id.Base58())
	}

	return
}

// String returns a human-readable Version of the BlockIDs.
func (m BlockIDs) String() string {
	if len(m) == 0 {
		return "BlockIDs{}"
	}

	result := "BlockIDs{\n"
	for blockID := range m {
		result += strings.Repeat(" ", stringify.INDENTATION_SIZE) + blockID.String() + ",\n"
	}
	result += "}"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Block //////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// LastValidBlockType counts StrongParents, WeakParents, ShallowLikeParents.
	LastValidBlockType = ShallowLikeParentType
)

// Block represents the core block for the base layer Tangle.
type Block struct {
	model.Storable[BlockID, Block, *Block, BlockModel] `serix:"0"`
	payload                                            payload.Payload
}
type BlockModel struct {
	// core properties (get sent over the wire)
	Version              uint8             `serix:"0"`
	Parents              ParentBlockIDs    `serix:"1"`
	IssuerPublicKey      ed25519.PublicKey `serix:"2"`
	IssuingTime          time.Time         `serix:"3"`
	SequenceNumber       uint64            `serix:"4"`
	PayloadBytes         []byte            `serix:"5,lengthPrefixType=uint32"`
	EI                   epoch.Index       `serix:"6"`
	ECR                  epoch.ECR         `serix:"7"`
	PrevEC               epoch.EC          `serix:"8"`
	LatestConfirmedEpoch epoch.Index       `serix:"9"`
	Nonce                uint64            `serix:"10"`
	Signature            ed25519.Signature `serix:"11"`
}

// NewBlock creates a new block with the details provided by the issuer.
func NewBlock(references ParentBlockIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, blkPayload payload.Payload, nonce uint64, signature ed25519.Signature,
	latestConfirmedEpoch epoch.Index, ecRecord *epoch.ECRecord, versionOpt ...uint8,
) *Block {
	version := BlockVersion
	if len(versionOpt) == 1 {
		version = versionOpt[0]
	}
	blk := model.NewStorable[BlockID, Block](&BlockModel{
		Version:              version,
		Parents:              references,
		IssuerPublicKey:      issuerPublicKey,
		IssuingTime:          issuingTime,
		SequenceNumber:       sequenceNumber,
		PayloadBytes:         lo.PanicOnErr(blkPayload.Bytes()),
		EI:                   ecRecord.EI(),
		ECR:                  ecRecord.ECR(),
		PrevEC:               ecRecord.PrevEC(),
		LatestConfirmedEpoch: latestConfirmedEpoch,
		Nonce:                nonce,
		Signature:            signature,
	})
	blk.payload = blkPayload

	return blk
}

// NewBlockWithValidation creates a new block while performing ths following syntactical checks:
// 1. A Strong Parents Block must exist.
// 2. Parents Block types cannot repeat.
// 3. Parent count per block 1 <= x <= 8.
// 4. Parents unique within block.
// 5. Parents lexicographically sorted within block.
// 7. Blocks should be ordered by type in ascending order.
// 6. A Parent(s) repetition is only allowed when it occurs across Strong and Like parents.
func NewBlockWithValidation(references ParentBlockIDs, issuingTime time.Time, issuerPublicKey ed25519.PublicKey,
	sequenceNumber uint64, blkPayload payload.Payload, nonce uint64, signature ed25519.Signature, latestConfirmedEpoch epoch.Index, epochCommitment *epoch.ECRecord, version ...uint8,
) (result *Block, err error) {
	blk := NewBlock(references, issuingTime, issuerPublicKey, sequenceNumber, blkPayload, nonce, signature, latestConfirmedEpoch, epochCommitment, version...)

	if _, err = blk.Bytes(); err != nil {
		return nil, errors.Errorf("failed to create block: %w", err)
	}
	return blk, nil
}

// VerifySignature verifies the Signature of the block.
func (m *Block) VerifySignature() (valid bool, err error) {
	blkBytes, err := m.Bytes()
	if err != nil {
		return false, errors.Errorf("failed to create block bytes: %w", err)
	}
	signature := m.Signature()

	contentLength := len(blkBytes) - len(signature)
	content := blkBytes[:contentLength]

	return m.M.IssuerPublicKey.VerifySignature(content, signature), nil
}

// IDBytes implements Element interface in scheduler NodeQueue that returns the BlockID of the block in bytes.
func (m *Block) IDBytes() []byte {
	return m.ID().Bytes()
}

// Version returns the block Version.
func (m *Block) Version() uint8 {
	return m.M.Version
}

// ParentsByType returns a slice of all parents of the desired type.
func (m *Block) ParentsByType(parentType ParentsType) BlockIDs {
	if parents, ok := m.M.Parents[parentType]; ok {
		return parents.Clone()
	}
	return NewBlockIDs()
}

// ForEachParent executes a consumer func for each parent.
func (m *Block) ForEachParent(consumer func(parent Parent)) {
	for parentType, parents := range m.M.Parents {
		for parentID := range parents {
			consumer(Parent{
				Type: parentType,
				ID:   parentID,
			})
		}
	}
}

func (m *Block) Parents() (parents []BlockID) {
	m.ForEachParent(func(parent Parent) {
		parents = append(parents, parent.ID)
	})
	return
}

// ForEachParentByType executes a consumer func for each strong parent.
func (m *Block) ForEachParentByType(parentType ParentsType, consumer func(parentBlockID BlockID) bool) {
	for parentID := range m.ParentsByType(parentType) {
		if !consumer(parentID) {
			return
		}
	}
}

// ParentsCountByType returns the total parents count of this block.
func (m *Block) ParentsCountByType(parentType ParentsType) uint8 {
	return uint8(len(m.ParentsByType(parentType)))
}

// IssuerPublicKey returns the public key of the block issuer.
func (m *Block) IssuerPublicKey() ed25519.PublicKey {
	return m.M.IssuerPublicKey
}

// IssuingTime returns the time when this block was created.
func (m *Block) IssuingTime() time.Time {
	return m.M.IssuingTime
}

// SequenceNumber returns the sequence number of this block.
func (m *Block) SequenceNumber() uint64 {
	return m.M.SequenceNumber
}

// Payload returns the Payload of the block.
func (m *Block) Payload() payload.Payload {
	m.Lock()
	defer m.Unlock()

	if m.payload == nil {
		_, err := serix.DefaultAPI.Decode(context.Background(), m.M.PayloadBytes, &m.payload, serix.WithValidation())
		if err != nil {
			panic(err)
		}

		if m.payload.Type() == devnetvm.TransactionType {
			tx := m.payload.(*devnetvm.Transaction)
			tx.SetID(utxo.NewTransactionID(m.M.PayloadBytes))

			devnetvm.SetOutputID(tx.Essence(), tx.ID())
		}
	}

	return m.payload
}

// Nonce returns the Nonce of the block.
func (m *Block) Nonce() uint64 {
	return m.M.Nonce
}

// EI returns the EI of the block.
func (m *Block) EI() epoch.Index {
	return m.M.EI
}

// ECR returns the ECR of the block.
func (m *Block) ECR() epoch.ECR {
	return m.M.ECR
}

// PrevEC returns the PrevEC of the block.
func (m *Block) PrevEC() epoch.EC {
	return m.M.PrevEC
}

// LatestConfirmedEpoch returns the LatestConfirmedEpoch of the block.
func (m *Block) LatestConfirmedEpoch() epoch.Index {
	return m.M.LatestConfirmedEpoch
}

// Signature returns the Signature of the block.
func (m *Block) Signature() ed25519.Signature {
	return m.M.Signature
}

// DetermineID calculates and sets the block's BlockID.
func (m *Block) DetermineID() (err error) {
	b, err := m.Bytes()
	if err != nil {
		return errors.Errorf("failed to determine block ID: %w", err)
	}

	m.SetID(NewBlockID(blake2b.Sum256(b), epoch.IndexFromTime(m.IssuingTime())))
	return nil
}

// Size returns the block size in bytes.
func (m *Block) Size() int {
	return len(lo.PanicOnErr(m.Bytes()))
}

func (m *Block) String() string {
	builder := stringify.StructBuilder("Block", stringify.StructField("id", m.ID()))

	for index, parent := range sortParents(m.ParentsByType(StrongParentType)) {
		builder.AddField(stringify.StructField(fmt.Sprintf("strongParent%d", index), parent.String()))
	}
	for index, parent := range sortParents(m.ParentsByType(WeakParentType)) {
		builder.AddField(stringify.StructField(fmt.Sprintf("weakParent%d", index), parent.String()))
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
func sortParents(parents BlockIDs) (sorted []BlockID) {
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
)

// String returns string representation of ParentsType.
func (bp ParentsType) String() string {
	return fmt.Sprintf("ParentType(%s)", []string{"Undefined", "Strong", "Weak", "Shallow Like"}[bp])
}

// Parent is a parent that can be either strong or weak.
type Parent struct {
	ID   BlockID
	Type ParentsType
}

// ParentBlockIDs is a map of ParentType to BlockIDs.
type ParentBlockIDs map[ParentsType]BlockIDs

// NewParentBlockIDs constructs a new ParentBlockIDs.
func NewParentBlockIDs() ParentBlockIDs {
	p := make(ParentBlockIDs)
	return p
}

// AddStrong adds a strong parent to the map.
func (p ParentBlockIDs) AddStrong(blockID BlockID) ParentBlockIDs {
	if _, exists := p[StrongParentType]; !exists {
		p[StrongParentType] = NewBlockIDs()
	}
	return p.Add(StrongParentType, blockID)
}

// Add adds a parent to the map.
func (p ParentBlockIDs) Add(parentType ParentsType, blockID BlockID) ParentBlockIDs {
	if _, exists := p[parentType]; !exists {
		p[parentType] = NewBlockIDs()
	}
	p[parentType].Add(blockID)
	return p
}

// AddAll adds a collection of parents to the map.
func (p ParentBlockIDs) AddAll(parentType ParentsType, blockIDs BlockIDs) ParentBlockIDs {
	if _, exists := p[parentType]; !exists {
		p[parentType] = NewBlockIDs()
	}
	p[parentType].AddAll(blockIDs)
	return p
}

// IsEmpty returns true if the ParentBlockIDs are empty.
func (p ParentBlockIDs) IsEmpty() bool {
	return p == nil || len(p) == 0
}

// Clone returns a copy of map.
func (p ParentBlockIDs) Clone() ParentBlockIDs {
	pCloned := NewParentBlockIDs()
	for parentType, blockIDs := range p {
		if _, exists := p[parentType]; !exists {
			p[parentType] = NewBlockIDs()
		}
		pCloned.AddAll(parentType, blockIDs)
	}
	return pCloned
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockMetadata ///////////////////////////////////////////////////////////////////////////////////////////////////////

type BlockMetadata struct {
	id                    BlockID
	missing               bool
	strongParents         BlockIDs
	weakParents           BlockIDs
	likedInsteadParents   BlockIDs
	strongChildren        []*BlockMetadata
	weakChildren          []*BlockMetadata
	likedInsteadChildren  []*BlockMetadata
	unsolidParentsCounter uint8
	solid                 bool
	invalid               bool

	*syncutils.StarvingMutex
}

func fullMetadataFromBlock(block *Block) func() *BlockMetadata {
	return func() *BlockMetadata {
		return &BlockMetadata{
			id:                   block.ID(),
			strongParents:        block.ParentsByType(StrongParentType),
			weakParents:          block.ParentsByType(WeakParentType),
			likedInsteadParents:  block.ParentsByType(ShallowLikeParentType),
			strongChildren:       make([]*BlockMetadata, 0),
			weakChildren:         make([]*BlockMetadata, 0),
			likedInsteadChildren: make([]*BlockMetadata, 0),
			StarvingMutex:        syncutils.NewStarvingMutex(),
		}
	}
}

func (b *BlockMetadata) Initialized() bool {
	return b.solid || b.unsolidParentsCounter != 0 || b.invalid
}

func (b *BlockMetadata) setInvalid() (updated bool) {
	b.Lock()
	defer b.Unlock()

	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

func (b *BlockMetadata) ParentIDs() BlockIDs {
	parents := b.strongParents.Clone()
	parents.AddAll(b.weakParents)
	parents.AddAll(b.likedInsteadParents)

	return parents
}

func (b *BlockMetadata) Children() (childrenMetadata []*BlockMetadata) {
	b.RLock()
	defer b.RUnlock()

	seenBlockIDs := make(map[BlockID]types.Empty)
	for _, parentsByType := range [][]*BlockMetadata{
		b.strongChildren,
		b.weakChildren,
		b.likedInsteadChildren,
	} {
		for _, childMetadata := range parentsByType {
			if _, exists := seenBlockIDs[childMetadata.id]; !exists {
				childrenMetadata = append(childrenMetadata, childMetadata)
				seenBlockIDs[childMetadata.id] = types.Void
			}
		}
	}

	return childrenMetadata
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// ErrNoStrongParents is triggered if there no strong parents.
	ErrNoStrongParents = errors.New("missing strong blocks in first parent block")
	// ErrBlockTypeIsUnknown is triggered when the block type is unknown.
	ErrBlockTypeIsUnknown = errors.Errorf("block types must range from %d-%d", 1, LastValidBlockType)
	// ErrConflictingReferenceAcrossBlocks is triggered if there conflicting references across blocks.
	ErrConflictingReferenceAcrossBlocks = errors.New("different blocks have conflicting references")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

var GenesisMetadata = &BlockMetadata{
	id:                   EmptyBlockID,
	strongParents:        make(BlockIDs),
	weakParents:          make(BlockIDs),
	likedInsteadParents:  make(BlockIDs),
	strongChildren:       make([]*BlockMetadata, 0),
	weakChildren:         make([]*BlockMetadata, 0),
	likedInsteadChildren: make([]*BlockMetadata, 0),
	solid:                true,
	StarvingMutex:        syncutils.NewStarvingMutex(),
}
