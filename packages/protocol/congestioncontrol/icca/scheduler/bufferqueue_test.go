package scheduler

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Buffered Queue test /////////////////////////////////////////////////////////////////////////////////////////////

const (
	numBlocks = 100
	maxBuffer = numBlocks
)

var (
	slotTimeProvider  = slot.NewTimeProvider(time.Now().Add(-5*time.Hour).Unix(), 10)
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
	noManaIdentity    = identity.GenerateIdentity()
	noManaNode        = identity.New(noManaIdentity.PublicKey())
	aMana             = int64(1)
)

func TestBufferQueue_Submit(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	var size int
	for i := 0; i < numBlocks; i++ {
		blk := newTestBlock(models.WithIssuer(identity.GenerateIdentity().PublicKey()))
		size++
		elements, err := b.Submit(blk, mockAccessManaRetriever)
		assert.Empty(t, elements)
		assert.NoError(t, err)
		assert.EqualValues(t, i+1, b.NumActiveIssuers())
		assert.EqualValues(t, i+1, ringLen(b))
	}
	assert.EqualValues(t, size, b.Size())
}

func TestBufferQueue_Unsubmit(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	blocks := make([]*Block, numBlocks)
	for i := range blocks {
		blocks[i] = newTestBlock(models.WithIssuer(identity.GenerateIdentity().PublicKey()))
		elements, err := b.Submit(blocks[i], mockAccessManaRetriever)
		assert.Empty(t, elements)
		assert.NoError(t, err)
	}
	assert.EqualValues(t, numBlocks, b.NumActiveIssuers())
	assert.EqualValues(t, numBlocks, ringLen(b))
	for i := range blocks {
		b.Unsubmit(blocks[i])
		assert.EqualValues(t, numBlocks, b.NumActiveIssuers())
		assert.EqualValues(t, numBlocks, ringLen(b))
	}
	assert.EqualValues(t, 0, b.Size())
}

// Drop unready blocks from the longest queue. Drop one block to fit new block. Drop two blocks to fit one new larger block.
func TestBufferQueue_SubmitWithDrop_Unready(t *testing.T) {
	b := NewBufferQueue(maxBuffer)
	preparedBlocks := make([]*Block, 0, 2*numBlocks)
	now := time.Now()
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlock(models.WithIssuer(noManaNode.PublicKey()), models.WithIssuingTime(now.Add(time.Duration(i)))))
	}
	for i := numBlocks / 2; i < numBlocks; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlock(models.WithIssuer(selfNode.PublicKey()), models.WithIssuingTime(now.Add(time.Duration(i)))))
	}
	for _, blk := range preparedBlocks {
		droppedBlocks, err := b.Submit(blk, mockAccessManaRetriever)
		assert.Empty(t, droppedBlocks)
		assert.NoError(t, err)
	}
	assert.EqualValues(t, 2, b.NumActiveIssuers())
	assert.EqualValues(t, 2, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// dropping single unready block
	droppedBlocks, err := b.Submit(newTestBlock(models.WithIssuer(selfNode.PublicKey())), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[0].ID(), droppedBlocks[0].ID())
	assert.LessOrEqual(t, maxBuffer, b.Size())

	// dropping two unready blocks to fit the new one
	droppedBlocks, err = b.Submit(newTestBlock(models.WithIssuer(selfNode.PublicKey()), models.WithPayload(payload.NewGenericDataPayload(make([]byte, 44)))), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[1].ID(), droppedBlocks[0].ID())
	assert.EqualValues(t, maxBuffer, b.Size())
}

// Drop newly submitted block because the node doesn't have enough access mana to send such a big block when the buffer is full.
func TestBufferQueue_SubmitWithDrop_DropNewBlock(t *testing.T) {
	b := NewBufferQueue(maxBuffer)
	preparedBlocks := make([]*Block, 0, 2*numBlocks)
	for i := 0; i < numBlocks; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlock(models.WithIssuer(selfNode.PublicKey()), models.WithSequenceNumber(uint64(i))))
	}
	for _, blk := range preparedBlocks {
		droppedBlocks, err := b.Submit(blk, mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, droppedBlocks)
	}
	assert.EqualValues(t, 1, b.NumActiveIssuers())
	assert.EqualValues(t, 1, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// drop newly submitted block when all blocks in the buffer are not ready
	newBlock := newTestBlock(models.WithIssuer(noManaNode.PublicKey()), models.WithPayload(payload.NewGenericDataPayload(make([]byte, 40))))
	droppedBlocks, err := b.Submit(newBlock, mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, newBlock.ID(), droppedBlocks[0].ID())
	assert.Equal(t, maxBuffer, b.Size())
}

// Drop ready blocks from the longest queue. Drop one ready block to fit new block. Drop two ready blocks to fit one new larger block.
func TestBufferQueue_SubmitWithDrop_Ready(t *testing.T) {
	b := NewBufferQueue(maxBuffer)
	preparedBlocks := make([]*Block, 0, 2*numBlocks)
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlock(models.WithIssuer(noManaNode.PublicKey())))
	}
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlock(models.WithIssuer(selfNode.PublicKey())))
	}
	for _, blk := range preparedBlocks {
		droppedBlocks, err := b.Submit(blk, mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, droppedBlocks)
		b.Ready(blk)
	}
	assert.EqualValues(t, 2, b.NumActiveIssuers())
	assert.EqualValues(t, 2, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// drop single ready block
	droppedBlocks, err := b.Submit(newTestBlock(models.WithIssuer(selfNode.PublicKey())), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[0].ID(), droppedBlocks[0].ID())
	assert.LessOrEqual(t, maxBuffer, b.Size())

	// drop two ready blocks to fit the newly submitted one
	droppedBlocks, err = b.Submit(newTestBlock(models.WithIssuer(selfNode.PublicKey()), models.WithPayload(payload.NewGenericDataPayload(make([]byte, 44)))), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[1].ID(), droppedBlocks[0].ID())
	assert.EqualValues(t, maxBuffer, b.Size())
}

func TestBufferQueue_Ready(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	blocks := make([]*Block, numBlocks)
	for i := range blocks {
		blocks[i] = newTestBlock(models.WithIssuer(identity.GenerateIdentity().PublicKey()))
		elements, err := b.Submit(blocks[i], mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, elements)
	}
	for i := range blocks {
		assert.True(t, b.Ready(blocks[i]))
		assert.False(t, b.Ready(blocks[i]))
		for ; b.Current().Size() == 0; b.Next() {
		}

		assert.Equal(t, blocks[i], b.PopFront())
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Time(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	future := newTestBlock(models.WithIssuer(selfNode.PublicKey()), models.WithIssuingTime(time.Now().Add(time.Second)))
	elements, err := b.Submit(future, mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Empty(t, elements)
	assert.True(t, b.Ready(future))

	now := newTestBlock(models.WithIssuer(selfNode.PublicKey()))
	elements, err = b.Submit(now, mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Empty(t, elements)
	assert.True(t, b.Ready(now))

	assert.Equal(t, now, b.PopFront())
	assert.Equal(t, future, b.PopFront())

	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Ring(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	blocks := make([]*Block, numBlocks)
	for i := range blocks {
		blocks[i] = newTestBlock(models.WithIssuer(identity.GenerateIdentity().PublicKey()))
		elements, err := b.Submit(blocks[i], mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, elements)
		assert.True(t, b.Ready(blocks[i]))
	}
	for i := range blocks {
		assert.Equal(t, blocks[i], b.Current().Front())
		b.Next()
	}
	assert.Equal(t, blocks[0], b.Current().Front())
}

func TestBufferQueue_IDs(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	assert.Empty(t, b.IDs())

	ids := make([]models.BlockID, numBlocks)
	for i := range ids {
		blk := newTestBlock(models.WithIssuer(identity.GenerateIdentity().PublicKey()))
		elements, err := b.Submit(blk, mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, elements)
		if i%2 == 0 {
			assert.True(t, b.Ready(blk))
		}
		ids[i] = blk.ID()
	}
	assert.ElementsMatch(t, ids, b.IDs())
}

func TestBufferQueue_InsertNode(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	otherNode := identity.GenerateIdentity()
	b.InsertIssuer(selfNode.ID())
	assert.Equal(t, selfNode.ID(), b.Current().IssuerID())
	assert.Equal(t, 0, b.Current().Size())

	b.InsertIssuer(otherNode.ID())
	b.Next()
	assert.Equal(t, otherNode.ID(), b.Current().IssuerID())
	assert.Equal(t, 0, b.Current().Size())
}

func TestBufferQueue_RemoveNode(t *testing.T) {
	b := NewBufferQueue(maxBuffer)

	elements, err := b.Submit(newTestBlock(models.WithIssuer(selfNode.PublicKey())), mockAccessManaRetriever)
	assert.Empty(t, elements)
	assert.NoError(t, err)

	otherNode := identity.GenerateIdentity()
	elements, err = b.Submit(newTestBlock(models.WithIssuer(otherNode.PublicKey())), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Empty(t, elements)

	assert.Equal(t, selfNode.ID(), b.Current().IssuerID())
	b.RemoveIssuer(selfNode.ID())
	assert.Equal(t, otherNode.ID(), b.Current().IssuerID())
	b.RemoveIssuer(otherNode.ID())
	assert.Nil(t, b.Current())
}

func ringLen(b *BufferQueue) int {
	n := 0
	if q := b.Current(); q != nil {
		n = 1
		for b.Next() != q {
			n++
		}
	}
	return n
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func newTestBlock(opts ...options.Option[models.Block]) *Block {
	parents := models.NewParentBlockIDs()
	parents.AddStrong(models.EmptyBlockID)
	opts = append(opts, models.WithParents(parents))

	blk := NewBlock(booker.NewBlock(blockdag.NewBlock(models.NewBlock(opts...))))
	if err := blk.DetermineID(slotTimeProvider); err != nil {
		panic(errors.Wrap(err, "could not determine BlockID"))
	}

	return blk
}

// mockAccessManaRetriever returns mocked access mana value for a node.
func mockAccessManaRetriever(id identity.ID) int64 {
	if id == selfNode.ID() {
		return aMana
	}
	return 0
}
