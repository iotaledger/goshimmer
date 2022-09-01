package schedulerutils_test

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/byteutils"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/schedulerutils"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

// region Buffered Queue test /////////////////////////////////////////////////////////////////////////////////////////////

const (
	numBlocks = 100
	maxBuffer = numBlocks
	maxQueue  = 2 * maxBuffer / numBlocks
)

var (
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
	noManaIdentity    = identity.GenerateIdentity()
	noManaNode        = identity.New(noManaIdentity.PublicKey())
	aMana             = 1.0
)

func TestBufferQueue_Submit(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	var size int
	for i := 0; i < numBlocks; i++ {
		blk := newTestBlockWithIndex(identity.GenerateIdentity().PublicKey(), i)
		size++
		elements, err := b.Submit(blk, mockAccessManaRetriever)
		assert.Empty(t, elements)
		assert.NoError(t, err)
		assert.EqualValues(t, i+1, b.NumActiveNodes())
		assert.EqualValues(t, i+1, ringLen(b))
	}
	assert.EqualValues(t, size, b.Size())
}

func TestBufferQueue_Unsubmit(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	blocks := make([]*testBlock, numBlocks)
	for i := range blocks {
		blocks[i] = newTestBlockWithIndex(identity.GenerateIdentity().PublicKey(), i)
		elements, err := b.Submit(blocks[i], mockAccessManaRetriever)
		assert.Empty(t, elements)
		assert.NoError(t, err)
	}
	assert.EqualValues(t, numBlocks, b.NumActiveNodes())
	assert.EqualValues(t, numBlocks, ringLen(b))
	for i := range blocks {
		b.Unsubmit(blocks[i])
		assert.EqualValues(t, numBlocks, b.NumActiveNodes())
		assert.EqualValues(t, numBlocks, ringLen(b))
	}
	assert.EqualValues(t, 0, b.Size())
}

// Drop unready blocks from the longest queue. Drop one block to fit new block. Drop two blocks to fit one new larger block.
func TestBufferQueue_SubmitWithDrop_Unready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)
	preparedBlocks := make([]*testBlock, 0, 2*numBlocks)
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlockWithIndex(noManaNode.PublicKey(), i))
	}
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlockWithIndex(selfNode.PublicKey(), i))
	}
	for _, blk := range preparedBlocks {
		droppedBlocks, err := b.Submit(blk, mockAccessManaRetriever)
		assert.Empty(t, droppedBlocks)
		assert.NoError(t, err)
	}
	assert.EqualValues(t, 2, b.NumActiveNodes())
	assert.EqualValues(t, 2, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// dropping single unready block
	droppedBlocks, err := b.Submit(newTestBlockWithIndex(selfNode.PublicKey(), 0), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[0].IDBytes(), droppedBlocks[0][:])
	assert.LessOrEqual(t, maxBuffer, b.Size())

	// dropping two unready blocks to fit the new one
	droppedBlocks, err = b.Submit(newLargeTestBlock(selfNode.PublicKey(), make([]byte, 44)), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[1].IDBytes(), droppedBlocks[0][:])
	assert.EqualValues(t, maxBuffer, b.Size())
}

// Drop newly submitted block because the node doesn't have enough access mana to send such a big block when the buffer is full.
func TestBufferQueue_SubmitWithDrop_DropNewBlock(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)
	preparedBlocks := make([]*testBlock, 0, 2*numBlocks)
	for i := 0; i < numBlocks; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlockWithIndex(selfNode.PublicKey(), i))
	}
	for _, blk := range preparedBlocks {
		droppedBlocks, err := b.Submit(blk, mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, droppedBlocks)
	}
	assert.EqualValues(t, 1, b.NumActiveNodes())
	assert.EqualValues(t, 1, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// drop newly submitted block when all blocks in the buffer are not ready
	newBlock := newLargeTestBlock(noManaNode.PublicKey(), make([]byte, 40))
	droppedBlocks, err := b.Submit(newBlock, mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, newBlock.IDBytes(), droppedBlocks[0][:])
	assert.Equal(t, maxBuffer, b.Size())
}

// Drop ready blocks from the longest queue. Drop one ready block to fit new block. Drop two ready blocks to fit one new larger block.
func TestBufferQueue_SubmitWithDrop_Ready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)
	preparedBlocks := make([]*testBlock, 0, 2*numBlocks)
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlockWithIndex(noManaNode.PublicKey(), i))
	}
	for i := 0; i < numBlocks/2; i++ {
		preparedBlocks = append(preparedBlocks, newTestBlockWithIndex(selfNode.PublicKey(), i))
	}
	for _, blk := range preparedBlocks {
		droppedBlocks, err := b.Submit(blk, mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, droppedBlocks)
		b.Ready(blk)
	}
	assert.EqualValues(t, 2, b.NumActiveNodes())
	assert.EqualValues(t, 2, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// drop single ready block
	droppedBlocks, err := b.Submit(newTestBlockWithIndex(selfNode.PublicKey(), 0), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[0].IDBytes(), droppedBlocks[0][:])
	assert.LessOrEqual(t, maxBuffer, b.Size())

	// drop two ready blocks to fit the newly submitted one
	droppedBlocks, err = b.Submit(newLargeTestBlock(selfNode.PublicKey(), make([]byte, 44)), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Len(t, droppedBlocks, 1)
	assert.Equal(t, preparedBlocks[1].IDBytes(), droppedBlocks[0][:])
	assert.EqualValues(t, maxBuffer, b.Size())
}

func TestBufferQueue_Ready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	blocks := make([]*testBlock, numBlocks)
	for i := range blocks {
		blocks[i] = newTestBlockWithIndex(identity.GenerateIdentity().PublicKey(), i)
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
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	future := newTestBlock(selfNode.PublicKey())
	future.issuingTime = time.Now().Add(time.Second)
	elements, err := b.Submit(future, mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Empty(t, elements)
	assert.True(t, b.Ready(future))

	now := newTestBlock(selfNode.PublicKey())
	elements, err = b.Submit(now, mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Empty(t, elements)
	assert.True(t, b.Ready(now))

	assert.Equal(t, now, b.PopFront())
	assert.Equal(t, future, b.PopFront())

	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Ring(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	blocks := make([]*testBlock, numBlocks)
	for i := range blocks {
		blocks[i] = newTestBlockWithIndex(identity.GenerateIdentity().PublicKey(), i)
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
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	assert.Empty(t, b.IDs())

	ids := make([]schedulerutils.ElementID, numBlocks)
	for i := range ids {
		blk := newTestBlockWithIndex(identity.GenerateIdentity().PublicKey(), i)
		elements, err := b.Submit(blk, mockAccessManaRetriever)
		assert.NoError(t, err)
		assert.Empty(t, elements)
		if i%2 == 0 {
			assert.True(t, b.Ready(blk))
		}
		ids[i] = schedulerutils.ElementIDFromBytes(blk.IDBytes())
	}
	assert.ElementsMatch(t, ids, b.IDs())
}

func TestBufferQueue_InsertNode(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	otherNode := identity.GenerateIdentity()
	b.InsertNode(selfNode.ID())
	assert.Equal(t, selfNode.ID(), b.Current().NodeID())
	assert.Equal(t, 0, b.Current().Size())

	b.InsertNode(otherNode.ID())
	b.Next()
	assert.Equal(t, otherNode.ID(), b.Current().NodeID())
	assert.Equal(t, 0, b.Current().Size())
}

func TestBufferQueue_RemoveNode(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	elements, err := b.Submit(newTestBlock(selfNode.PublicKey()), mockAccessManaRetriever)
	assert.Empty(t, elements)
	assert.NoError(t, err)

	otherNode := identity.GenerateIdentity()
	elements, err = b.Submit(newTestBlock(otherNode.PublicKey()), mockAccessManaRetriever)
	assert.NoError(t, err)
	assert.Empty(t, elements)

	assert.Equal(t, selfNode.ID(), b.Current().NodeID())
	b.RemoveNode(selfNode.ID())
	assert.Equal(t, otherNode.ID(), b.Current().NodeID())
	b.RemoveNode(otherNode.ID())
	assert.Nil(t, b.Current())
}

func ringLen(b *schedulerutils.BufferQueue) int {
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

type testBlock struct {
	pubKey      ed25519.PublicKey
	issuingTime time.Time
	payload     []byte
	bytes       []byte
	idx         int
}

func newTestBlockWithIndex(pubKey ed25519.PublicKey, index int) *testBlock {
	return &testBlock{
		pubKey:      pubKey,
		idx:         index,
		issuingTime: time.Now(),
	}
}

func newTestBlock(pubKey ed25519.PublicKey) *testBlock {
	return &testBlock{
		pubKey:      pubKey,
		issuingTime: time.Now(),
	}
}

func newLargeTestBlock(pubKey ed25519.PublicKey, payload []byte) *testBlock {
	return &testBlock{
		pubKey:      pubKey,
		issuingTime: time.Now(),
		payload:     payload,
	}
}

func (m *testBlock) IDBytes() []byte {
	tmp := blake2b.Sum256(m.Bytes())
	return byteutils.ConcatBytes(tmp[:], marshalutil.New(marshalutil.Int64Size).WriteInt64(int64(m.idx)).Bytes())
}

func (m *testBlock) Bytes() []byte {
	if m.bytes != nil {
		return m.bytes
	}
	// marshal result
	marshalUtil := marshalutil.New()
	marshalUtil.Write(m.pubKey)
	marshalUtil.WriteTime(m.issuingTime)
	marshalUtil.WriteBytes(m.payload)
	marshalUtil.WriteInt32(int32(m.idx))
	m.bytes = marshalUtil.Bytes()

	return m.bytes
}

func (m *testBlock) Size() int {
	return len(m.Bytes())
}

func (m *testBlock) IssuerPublicKey() ed25519.PublicKey {
	return m.pubKey
}

func (m *testBlock) IssuingTime() time.Time {
	return m.issuingTime
}

// mockAccessManaRetriever returns mocked access mana value for a node.
func mockAccessManaRetriever(id identity.ID) float64 {
	if id == selfNode.ID() {
		return aMana
	}
	return 0
}
