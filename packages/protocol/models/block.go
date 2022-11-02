package models

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/stringify"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

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
	size                                          *int
}

type block struct {
	// core properties (get sent over the wire)
	Version              uint8                  `serix:"0"`
	Parents              ParentBlockIDs         `serix:"1"`
	IssuerPublicKey      ed25519.PublicKey      `serix:"2"`
	IssuingTime          time.Time              `serix:"3"`
	SequenceNumber       uint64                 `serix:"4"`
	PayloadBytes         []byte                 `serix:"5,lengthPrefixType=uint32"`
	EpochCommitment      *commitment.Commitment `serix:"6"`
	LatestConfirmedEpoch epoch.Index            `serix:"7"`
	Nonce                uint64                 `serix:"8"`
	Signature            ed25519.Signature      `serix:"9"`
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
		EpochCommitment: commitment.New(0, commitment.ID{}, types.Identifier{}, 0),
	})
	blk.payload = defaultPayload

	return options.Apply(blk, opts)
}

func NewEmptyBlock(id BlockID, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(model.NewStorable[BlockID, Block](&block{}), opts, func(b *Block) {
		b.SetID(id)
		b.M.PayloadBytes = lo.PanicOnErr(payload.NewGenericDataPayload([]byte("")).Bytes())
	})
}

// VerifySignature verifies the Signature of the block.
func (b *Block) VerifySignature() (valid bool, err error) {
	blkBytes, err := b.Bytes()
	if err != nil {
		return false, errors.Errorf("failed to create block bytes: %w", err)
	}
	signature := b.Signature()

	contentLength := len(blkBytes) - len(signature)
	content := blkBytes[:contentLength]

	return b.M.IssuerPublicKey.VerifySignature(content, signature), nil
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
	return b.M.EpochCommitment
}

// LatestConfirmedEpoch returns the LatestConfirmedEpoch of the block.
func (b *Block) LatestConfirmedEpoch() epoch.Index {
	return b.M.LatestConfirmedEpoch
}

// Signature returns the Signature of the block.
func (b *Block) Signature() ed25519.Signature {
	return b.M.Signature
}

func (b *Block) SetSignature(signature ed25519.Signature) {
	b.M.Signature = signature
}

// DetermineID calculates and sets the block's BlockID and size.
func (b *Block) DetermineID() (err error) {
	buf, err := b.Bytes()
	if err != nil {
		return errors.Errorf("failed to determine block ID: %w", err)
	}

	b.DetermineIDFromBytes(buf)
	return nil
}

// DetermineIDFromBytes calculates and sets the block's BlockID and size.
func (b *Block) DetermineIDFromBytes(buf []byte) {
	b.SetID(NewBlockID(blake2b.Sum256(buf), epoch.IndexFromTime(b.IssuingTime())))

	b.Lock()
	defer b.Unlock()
	l := len(buf)
	b.size = &l
}

func (b *Block) SetSize(size int) {
	b.size = &size
}

// Size returns the block size in bytes.
func (b *Block) Size() int {
	b.RLock()
	defer b.RUnlock()

	if b.size == nil {
		panic(fmt.Sprintf("size is not set for %s", b.ID()))
	}

	return *b.size
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

func WithPayload(payload payload.Payload) options.Option[Block] {
	return func(m *Block) {
		m.payload = payload
		m.M.PayloadBytes = lo.PanicOnErr(payload.Bytes())
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

func WithLatestConfirmedEpoch(epoch epoch.Index) options.Option[Block] {
	return func(b *Block) {
		b.M.LatestConfirmedEpoch = epoch
	}
}

func WithCommitment(commitment *commitment.Commitment) options.Option[Block] {
	return func(b *Block) {
		b.M.EpochCommitment = commitment
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
