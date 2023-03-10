package models

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// BlockVersion defines the Version of the block structure.
	BlockVersion uint8 = 1

	// MaxBlockSize defines the maximum size of a block in bytes.
	MaxBlockSize = 64 * 1024

	// MaxBlockWork defines the maximum work of a block.
	MaxBlockWork = 1

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

// region Block //////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// LastValidBlockType counts StrongParents, WeakParents, ShallowLikeParents.
	LastValidBlockType = ShallowLikeParentType
)

// Block represents the core block for the base layer Tangle.
type Block struct {
	model.Storable[BlockID, Block, *Block, block] `serix:"0"`
	payload                                       payload.Payload
	issuerID                                      *identity.ID
}

type block struct {
	// core properties (get sent over the wire)
	Version             uint8                  `serix:"0"`
	Parents             ParentBlockIDs         `serix:"1"`
	IssuerPublicKey     ed25519.PublicKey      `serix:"2"`
	IssuingTime         time.Time              `serix:"3"`
	SequenceNumber      uint64                 `serix:"4"`
	PayloadBytes        []byte                 `serix:"5,lengthPrefixType=uint32"`
	SlotCommitment      *commitment.Commitment `serix:"6"`
	LatestConfirmedSlot slot.Index             `serix:"7"`
	Nonce               uint64                 `serix:"8"`
	Signature           ed25519.Signature      `serix:"9"`
}

// NewBlock creates a new block with the details provided by the issuer.
func NewBlock(opts ...options.Option[Block]) *Block {
	defaultPayload := payload.NewGenericDataPayload([]byte(""))

	blk := model.NewStorable[BlockID, Block](&block{
		Version:         BlockVersion,
		Parents:         NewParentBlockIDs(),
		IssuerPublicKey: ed25519.GenerateKeyPair().PublicKey,
		IssuingTime:     time.Now(),
		SequenceNumber:  0,
		PayloadBytes:    lo.PanicOnErr(defaultPayload.Bytes()),
		SlotCommitment:  commitment.New(0, commitment.ID{}, types.Identifier{}, 0),
	})
	blk.payload = defaultPayload

	return options.Apply(blk, opts)
}

func NewEmptyBlock(id BlockID, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(model.NewStorable[BlockID, Block](&block{}), opts, func(b *Block) {
		b.SetID(id)
		b.M.PayloadBytes = lo.PanicOnErr(payload.NewGenericDataPayload([]byte("")).Bytes())
		b.M.SlotCommitment = commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	})
}

func (b *Block) ContentHash() (contentHash types.Identifier, err error) {
	blkBytes, err := b.Bytes()
	if err != nil {
		return types.Identifier{}, errors.Wrap(err, "failed to create block bytes")
	}

	return blake2b.Sum256(blkBytes[:len(blkBytes)-ed25519.SignatureSize]), nil
}

func (b *Block) Sign(pair *ed25519.KeyPair) error {
	b.M.IssuerPublicKey = pair.PublicKey

	contentHash, err := b.ContentHash()
	if err != nil {
		return errors.Wrap(err, "failed to obtain block content's hash")
	}

	issuingTimeBytes, err := serix.DefaultAPI.Encode(context.Background(), b.IssuingTime(), serix.WithValidation())
	if err != nil {
		return errors.Wrap(err, "failed to serialize block's issuing time")
	}

	commitmentIDBytes, err := b.Commitment().ID().Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block's commitment ID")
	}

	b.SetSignature(pair.PrivateKey.Sign(byteutils.ConcatBytes(issuingTimeBytes, commitmentIDBytes, contentHash[:])))
	return nil
}

// VerifySignature verifies the Signature of the block.
func (b *Block) VerifySignature() (valid bool, err error) {
	contentHash, err := b.ContentHash()
	if err != nil {
		return false, errors.Wrap(err, "failed to obtain block content's hash")
	}

	issuingTimeBytes, err := serix.DefaultAPI.Encode(context.Background(), b.IssuingTime(), serix.WithValidation())
	if err != nil {
		return false, errors.Wrap(err, "failed to serialize block's issuing time")
	}

	commitmentIDBytes, err := b.Commitment().ID().Bytes()
	if err != nil {
		return false, errors.Wrap(err, "failed to serialize block's commitment ID")
	}

	return b.M.IssuerPublicKey.VerifySignature(byteutils.ConcatBytes(issuingTimeBytes, commitmentIDBytes, contentHash[:]), b.Signature()), nil
}

// Version returns the block Version.
func (b *Block) Version() uint8 {
	return b.M.Version
}

// ParentsByType returns a slice of all parents of the desired type.
func (b *Block) ParentsByType(parentType ParentsType) BlockIDs {
	if parents, ok := b.M.Parents[parentType]; ok {
		return parents.Clone()
	}
	return NewBlockIDs()
}

// ForEachParent executes a consumer func for each parent.
func (b *Block) ForEachParent(consumer func(parent Parent)) {
	b.M.Parents.ForEach(consumer)
}

// Parents returns a copy of the parents of the block.
func (b *Block) Parents() (parents []BlockID) {
	b.ForEachParent(func(parent Parent) {
		parents = append(parents, parent.ID)
	})
	return
}

// StrongParents returns a copy of the strong parents of the block.
func (b *Block) StrongParents() (strongParents []BlockID) {
	return b.ParentsByType(StrongParentType).Slice()
}

// ForEachParentByType executes a consumer func for each strong parent.
func (b *Block) ForEachParentByType(parentType ParentsType, consumer func(parentBlockID BlockID) bool) {
	for parentID := range b.ParentsByType(parentType) {
		if !consumer(parentID) {
			return
		}
	}
}

// ParentsCountByType returns the total parents count of this block.
func (b *Block) ParentsCountByType(parentType ParentsType) uint8 {
	return uint8(len(b.ParentsByType(parentType)))
}

// IssuerPublicKey returns the public key of the block issuer.
func (b *Block) IssuerPublicKey() ed25519.PublicKey {
	return b.M.IssuerPublicKey
}

func (b *Block) IssuerID() (issuerID identity.ID) {
	b.RLock()
	if b.issuerID != nil {
		defer b.RUnlock()
		return *b.issuerID
	}
	b.RUnlock()

	b.Lock()
	defer b.Unlock()

	issuerID = identity.NewID(b.IssuerPublicKey())
	b.issuerID = &issuerID

	return
}

// IssuingTime returns the time when this block was created.
func (b *Block) IssuingTime() time.Time {
	return b.M.IssuingTime
}

// SequenceNumber returns the sequence number of this block.
func (b *Block) SequenceNumber() uint64 {
	return b.M.SequenceNumber
}

// Payload returns the Payload of the block.
func (b *Block) Payload() payload.Payload {
	b.Lock()
	defer b.Unlock()
	if b.payload == nil {
		_, err := serix.DefaultAPI.Decode(context.Background(), b.M.PayloadBytes, &b.payload, serix.WithValidation())
		if err != nil {
			panic(err)
		}

		if tx, isTransaction := b.payload.(utxo.Transaction); isTransaction {
			tx.SetID(utxo.NewTransactionID(b.M.PayloadBytes))
			if devnetTx, isDevnetTx := b.payload.(*devnetvm.Transaction); isDevnetTx {
				devnetvm.SetOutputID(devnetTx.Essence(), tx.ID())
			}
		}
	}

	return b.payload
}

// Nonce returns the Nonce of the block.
func (b *Block) Nonce() uint64 {
	return b.M.Nonce
}

// Commitment returns the Commitment of the block.
func (b *Block) Commitment() *commitment.Commitment {
	return b.M.SlotCommitment
}

// LatestConfirmedSlot returns the LatestConfirmedSlot of the block.
func (b *Block) LatestConfirmedSlot() slot.Index {
	return b.M.LatestConfirmedSlot
}

// Signature returns the Signature of the block.
func (b *Block) Signature() ed25519.Signature {
	return b.M.Signature
}

func (b *Block) SetSignature(signature ed25519.Signature) {
	b.M.Signature = signature
	b.InvalidateBytesCache()
}

// DetermineID calculates and sets the block's BlockID and size.
func (b *Block) DetermineID(slotTimeProvider *slot.TimeProvider, blockIdentifier ...types.Identifier) (err error) {
	blkBytes, err := b.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to create block bytes")
	}
	if len(blockIdentifier) > 0 {
		b.SetID(BlockID{
			Identifier: blockIdentifier[0],
			SlotIndex:  slotTimeProvider.IndexFromTime(b.IssuingTime()),
		})
	} else {
		b.SetID(DetermineID(blkBytes, slotTimeProvider.IndexFromTime(b.IssuingTime())))
	}

	return nil
}

// Size returns the block size in bytes.
func (b *Block) Size() int {
	return len(lo.PanicOnErr(b.Bytes()))
}

// Work returns the work units required to process this block.
// Currently to 1 for all blocks, but could be improved.
func (b *Block) Work() int {
	return 1
}

func (b *Block) String() string {
	builder := stringify.NewStructBuilder("Block", stringify.NewStructField("id", b.ID()))

	for index, parent := range sortParents(b.ParentsByType(StrongParentType)) {
		builder.AddField(stringify.NewStructField(fmt.Sprintf("strongParent%d", index), parent.String()))
	}
	for index, parent := range sortParents(b.ParentsByType(WeakParentType)) {
		builder.AddField(stringify.NewStructField(fmt.Sprintf("weakParent%d", index), parent.String()))
	}
	for index, parent := range sortParents(b.ParentsByType(ShallowLikeParentType)) {
		builder.AddField(stringify.NewStructField(fmt.Sprintf("shallowlikeParent%d", index), parent.String()))
	}

	builder.AddField(stringify.NewStructField("Issuer", b.IssuerPublicKey()))
	builder.AddField(stringify.NewStructField("IssuingTime", b.IssuingTime()))
	builder.AddField(stringify.NewStructField("SequenceNumber", b.SequenceNumber()))
	builder.AddField(stringify.NewStructField("Payload", b.Payload()))
	builder.AddField(stringify.NewStructField("Nonce", b.Nonce()))
	builder.AddField(stringify.NewStructField("Commitment", b.Commitment()))
	builder.AddField(stringify.NewStructField("Signature", b.Signature()))

	return builder.String()
}

// DetermineID calculates the block's BlockID.
func DetermineID(blkBytes []byte, slotIndex slot.Index) BlockID {
	contentHash := blake2b.Sum256(blkBytes[:len(blkBytes)-ed25519.SignatureSize])
	signatureBytes := blkBytes[len(blkBytes)-ed25519.SignatureSize:]

	return NewBlockID(contentHash, lo.Return1(ed25519.SignatureFromBytes(signatureBytes)), slotIndex)
}

// sortParents sorts given parents and returns a new slice with sorted parents.
func sortParents(parents BlockIDs) (sorted []BlockID) {
	sorted = parents.Slice()

	// sort parents
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(lo.PanicOnErr(sorted[i].Bytes()), lo.PanicOnErr(sorted[j].Bytes())) < 0
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithVersion(version uint8) options.Option[Block] {
	return func(m *Block) {
		m.M.Version = version
	}
}

func WithParents(parents ParentBlockIDs) options.Option[Block] {
	return func(b *Block) {
		b.M.Parents = parents
	}
}

func WithStrongParents(parents BlockIDs) options.Option[Block] {
	return func(block *Block) {
		if block.M.Parents == nil {
			block.M.Parents = NewParentBlockIDs()
		}
		block.M.Parents.AddAll(StrongParentType, parents)
	}
}

func WithWeakParents(parents BlockIDs) options.Option[Block] {
	return func(block *Block) {
		if block.M.Parents == nil {
			block.M.Parents = NewParentBlockIDs()
		}
		block.M.Parents.AddAll(WeakParentType, parents)
	}
}

func WithLikedInsteadParents(parents BlockIDs) options.Option[Block] {
	return func(block *Block) {
		if len(parents) == 0 {
			return
		}
		if block.M.Parents == nil {
			block.M.Parents = NewParentBlockIDs()
		}
		block.M.Parents.AddAll(ShallowLikeParentType, parents)
	}
}

func WithIssuingTime(issuingTime time.Time) options.Option[Block] {
	return func(m *Block) {
		m.M.IssuingTime = issuingTime
	}
}

func WithIssuer(issuer ed25519.PublicKey) options.Option[Block] {
	return func(m *Block) {
		m.M.IssuerPublicKey = issuer
	}
}

func WithSequenceNumber(sequenceNumber uint64) options.Option[Block] {
	return func(m *Block) {
		m.M.SequenceNumber = sequenceNumber
	}
}

func WithPayload(p payload.Payload) options.Option[Block] {
	return func(m *Block) {
		m.payload = p
		m.M.PayloadBytes = lo.PanicOnErr(p.Bytes())
	}
}

func WithSignature(signature ed25519.Signature) options.Option[Block] {
	return func(m *Block) {
		m.M.Signature = signature
	}
}

func WithNonce(nonce uint64) options.Option[Block] {
	return func(m *Block) {
		m.M.Nonce = nonce
	}
}

func WithLatestConfirmedSlot(index slot.Index) options.Option[Block] {
	return func(b *Block) {
		b.M.LatestConfirmedSlot = index
	}
}

func WithCommitment(cm *commitment.Commitment) options.Option[Block] {
	return func(b *Block) {
		b.M.SlotCommitment = cm
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
