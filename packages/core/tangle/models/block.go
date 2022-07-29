package models

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
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
	model.Storable[BlockID, Block, *Block, blockModel] `serix:"0"`
	payload                                            payload.Payload
}

type blockModel struct {
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
	blk := model.NewStorable[BlockID, Block](&blockModel{
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

func NewEmptyBlock(id BlockID) (newBlock *Block) {
	newBlock = model.NewStorable[BlockID, Block](&blockModel{})
	newBlock.SetID(id)

	return newBlock
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

// Parents returns a copy of the parents of the block.
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

// sortParents sorts given parents and returns a new slice with sorted parents.
func sortParents(parents BlockIDs) (sorted []BlockID) {
	sorted = parents.Slice()

	// sort parents
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].Bytes(), sorted[j].Bytes()) < 0
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
