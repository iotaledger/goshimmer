package tangleold

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

func randomBytes(size uint) []byte {
	buffer := make([]byte, size)
	_, err := rand.Read(buffer)
	if err != nil {
		panic(err)
	}
	return buffer
}

func randomBlockID() BlockID {
	var result BlockID
	err := result.FromRandomness()
	if err != nil {
		panic(err)
	}
	return result
}

func randomParents(count int) BlockIDs {
	parents := NewBlockIDs()
	for i := 0; i < count; i++ {
		parents.Add(randomBlockID())
	}
	return parents
}

func testSortParents(parents BlockIDs) []BlockID {
	parentsSorted := parents.Slice()
	sort.Slice(parentsSorted, func(i, j int) bool {
		return bytes.Compare(parentsSorted[i].Bytes(), parentsSorted[j].Bytes()) < 0
	})
	return parentsSorted
}

func TestNewBlockID(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		randID := randomBlockID()
		randIDString := randID.Base58()
		var blkID BlockID
		err := blkID.FromBase58(randIDString)
		assert.NoError(t, err)
		assert.Equal(t, randID, blkID)
	})

	t.Run("CASE: Not base58 encoded", func(t *testing.T) {
		var blkID BlockID
		err := blkID.FromBase58("O0l")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid base58 digit ('O')"))
		assert.Equal(t, EmptyBlockID, blkID)
	})
}

func TestBlockIDFromBytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		buffer := randomBytes(BlockIDLength)
		var blkID BlockID
		consumed, err := blkID.FromBytes(buffer)
		assert.NoError(t, err)
		assert.Equal(t, BlockIDLength, consumed)
		assert.Equal(t, blkID.Bytes(), buffer)
	})

	t.Run("CASE: Too few bytes", func(t *testing.T) {
		buffer := randomBytes(BlockIDLength - 1)
		var result BlockID
		_, err := result.FromBytes(buffer)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "not enough data for deserialization"))
	})

	t.Run("CASE: More bytes", func(t *testing.T) {
		buffer := randomBytes(BlockIDLength + 1)
		var result BlockID
		consumed, err := result.FromBytes(buffer)
		assert.NoError(t, err)
		assert.Equal(t, BlockIDLength, consumed)
		assert.Equal(t, buffer[:BlockIDLength], result.Bytes())
	})
}

func TestBlockID_Base58(t *testing.T) {
	randID := randomBlockID()
	randIDString := randID.Base58()
	fmt.Println(randIDString)
	assert.Equal(t, fmt.Sprintf("%s:%s", base58.Encode(randID.Identifier.Bytes()), strconv.FormatInt(int64(randID.EpochIndex), 10)), randIDString)
}

func TestBlock_VerifySignature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte("test"))

	unsigned := NewBlock(NewParentBlockIDs().AddStrong(EmptyBlockID), time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{}, 0, epoch.NewECRecord(0))
	assert.False(t, lo.PanicOnErr(unsigned.VerifySignature()))

	unsignedBytes, err := unsigned.Bytes()
	require.NoError(t, err)
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := NewBlock(NewParentBlockIDs().AddStrong(EmptyBlockID), time.Time{}, keyPair.PublicKey, 0, pl, 0, signature, 0, epoch.NewECRecord(0))
	assert.True(t, lo.PanicOnErr(signed.VerifySignature()))
}

func TestBlock_UnmarshalTransaction(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	references := ParentBlockIDs{
		StrongParentType: randomParents(1),
		WeakParentType:   randomParents(1),
	}

	testBlock, err := NewBlockWithValidation(references,
		time.Now(),
		ed25519.PublicKey{},
		0,
		randomTransaction(),
		0,
		ed25519.Signature{}, 0, epoch.NewECRecord(0))
	assert.NoError(t, err)
	assert.NoError(t, testBlock.DetermineID())

	restoredBlock := new(Block)
	err = restoredBlock.FromBytes(lo.PanicOnErr(testBlock.Bytes()))
	assert.NoError(t, err)
	assert.NoError(t, restoredBlock.DetermineID())
	assert.Equal(t, testBlock.ID(), restoredBlock.ID())
}

func TestBlock_MarshalUnmarshal(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testBlock, err := tangle.BlockFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
	require.NoError(t, err)
	assert.Equal(t, true, lo.PanicOnErr(testBlock.VerifySignature()))

	t.Log(testBlock)

	restoredBlock := new(Block)
	err = restoredBlock.FromBytes(lo.PanicOnErr(testBlock.Bytes()))
	assert.NoError(t, restoredBlock.DetermineID())

	if assert.NoError(t, err, err) {
		assert.Equal(t, testBlock.ID(), restoredBlock.ID())
		assert.Equal(t, testBlock.ParentsByType(StrongParentType), restoredBlock.ParentsByType(StrongParentType))
		assert.Equal(t, testBlock.ParentsByType(WeakParentType), restoredBlock.ParentsByType(WeakParentType))
		assert.Equal(t, testBlock.IssuerPublicKey(), restoredBlock.IssuerPublicKey())
		assert.Equal(t, testBlock.IssuingTime().Round(time.Second), restoredBlock.IssuingTime().Round(time.Second))
		assert.Equal(t, testBlock.SequenceNumber(), restoredBlock.SequenceNumber())
		assert.Equal(t, testBlock.Nonce(), restoredBlock.Nonce())
		assert.Equal(t, testBlock.Signature(), restoredBlock.Signature())
		assert.Equal(t, true, lo.PanicOnErr(restoredBlock.VerifySignature()))
	}
}

func TestNewBlockWithValidation(t *testing.T) {
	t.Run("CASE: Too many strong parents", func(t *testing.T) {
		// too many strong parents
		strongParents := testSortParents(randomParents(MaxParentsCount + 1))
		parentBlocks := NewParentBlockIDs()
		parentBlocks.AddAll(StrongParentType, NewBlockIDs(strongParents...))

		_, err := NewBlockWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0, epoch.NewECRecord(0), BlockVersion)
		assert.Error(t, err)
	})

	t.Run("CASE: Nil block", func(t *testing.T) {
		_, err := NewBlockWithValidation(
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.Error(t, err)
	})

	t.Run("CASE: Empty Block", func(t *testing.T) {
		_, err := NewBlockWithValidation(
			NewParentBlockIDs(),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.Error(t, err)
	})

	t.Run("CASE: Parent Blocks are unordered", func(t *testing.T) {
		parents := testSortParents(randomParents(1))
		parentBlocks := NewParentBlockIDs()
		parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))
		parentBlocks.AddAll(WeakParentType, NewBlockIDs(testSortParents(randomParents(MaxParentsCount))...))
		parentBlocks.AddAll(ShallowLikeParentType, NewBlockIDs(parents...))

		blk, err := NewBlockWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.NoError(t, err)
		assert.NoError(t, blk.DetermineID())
		blkBytes := lo.PanicOnErr(blk.Bytes())

		blkBytes[2] = byte(WeakParentType)
		blkBytes[2+BlockIDLength+2] = byte(StrongParentType)

		err = new(Block).FromObjectStorage(blk.IDBytes(), blkBytes)
		fmt.Println(err)
		assert.ErrorContains(t, err, "array elements must be in their lexical order (byte wise)")
	})

	t.Run("CASE: Repeating block types", func(t *testing.T) {
		// this can be tested only for deserialization as it's even impossible to create such block using current structures

		parents := testSortParents(randomParents(MaxParentsCount))
		parentBlocks := NewParentBlockIDs()
		parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))
		parentBlocks.AddAll(WeakParentType, NewBlockIDs(testSortParents(randomParents(MaxParentsCount))...))
		parentBlocks.AddAll(ShallowLikeParentType, NewBlockIDs(parents...))

		blk, err := NewBlockWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.NoError(t, blk.DetermineID())
		blkBytes := lo.PanicOnErr(blk.Bytes())

		blkBytes[2] = byte(WeakParentType)

		err = new(Block).FromObjectStorage(blk.IDBytes(), blkBytes)

		assert.ErrorContains(t, err, "array elements must be unique")
	})

	t.Run("CASE: Unknown block type", func(t *testing.T) {
		parents := testSortParents(randomParents(MaxParentsCount))

		parentBlocks := NewParentBlockIDs()
		parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))
		parentBlocks.AddAll(ShallowLikeParentType, NewBlockIDs(parents...))
		parentBlocks.AddAll(LastValidBlockType+1, NewBlockIDs(parents...))

		_, err := NewBlockWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.Error(t, err)
	})

	t.Run("Case: Duplicate references", func(t *testing.T) {
		parents := testSortParents(randomParents(4))

		parentBlocks := NewParentBlockIDs()
		parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))

		blk, err := NewBlockWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.NoError(t, blk.DetermineID())
		blkBytes := lo.PanicOnErr(blk.Bytes())

		// replace blockID with another one in the bytes output
		bytesOffset := 4
		copy(blkBytes[bytesOffset:bytesOffset+BlockIDLength], blkBytes[bytesOffset+BlockIDLength:bytesOffset+BlockIDLength+BlockIDLength])

		err = blk.FromObjectStorage(blk.IDBytes(), blkBytes)
		assert.ErrorContains(t, err, "array elements must be unique")
	})
	t.Run("Case: References out of order", func(t *testing.T) {
		parents := testSortParents(randomParents(4))

		parentBlocks := NewParentBlockIDs()
		parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))

		blk, err := NewBlockWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0, epoch.NewECRecord(0),
			BlockVersion,
		)
		assert.NoError(t, blk.DetermineID())

		blkBytes := lo.PanicOnErr(blk.Bytes())

		// replace parents in byte structure
		bytesOffset := 4
		parent3ByteOffset := bytesOffset + BlockIDLength + BlockIDLength
		copy(blkBytes[bytesOffset:bytesOffset+BlockIDLength], blkBytes[parent3ByteOffset:parent3ByteOffset+BlockIDLength])

		err = blk.FromObjectStorage(blk.IDBytes(), blkBytes)
		assert.ErrorContains(t, err, "array elements must be in their lexical order (byte wise)")
		// if the duplicates are not consecutive a lexicographically order error is returned
	})

	t.Run("Parents Repeating across blocks", func(t *testing.T) {
		parents := testSortParents(randomParents(4))

		{
			parentBlocks := NewParentBlockIDs()
			parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))
			parentBlocks.AddAll(ShallowLikeParentType, NewBlockIDs(parents...))

			_, err := NewBlockWithValidation(
				parentBlocks,
				time.Now(),
				ed25519.PublicKey{},
				0,

				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
				0,
				epoch.NewECRecord(0),
				BlockVersion,
			)
			assert.NoError(t, err, "strong and like parents may have duplicate parents")
		}

		{
			parentBlocks := NewParentBlockIDs()
			parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))
			parentBlocks.AddAll(WeakParentType, NewBlockIDs(parents...))

			_, err := NewBlockWithValidation(
				parentBlocks,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
				0,
				epoch.NewECRecord(0),
				BlockVersion,
			)
			assert.Error(t, err, "blocks in weak references may allow to overlap with strong references")
		}

		{
			// check for repeating block across weak and dislike block
			weakParents := testSortParents(randomParents(4))
			weakParents[2] = parents[2]

			parentBlocks := NewParentBlockIDs()
			parentBlocks.AddAll(StrongParentType, NewBlockIDs(parents...))
			parentBlocks.AddAll(WeakParentType, NewBlockIDs(weakParents...))

			_, err := NewBlockWithValidation(
				parentBlocks,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
				0,
				epoch.NewECRecord(0),
				BlockVersion)
			assert.Error(t, err)
		}
	})
}

func TestBlock_NewBlock(t *testing.T) {
	t.Run("CASE: No parents at all", func(t *testing.T) {
		_, err := NewBlockWithValidation(
			ParentBlockIDs{},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.ErrorIs(t, err, ErrNoStrongParents)
	})

	t.Run("CASE: Minimum number of parents", func(t *testing.T) {
		_, err := NewBlockWithValidation(
			emptyLikeReferencesFromStrongParents(NewBlockIDs(EmptyBlockID)),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		// should pass since EmptyBlockId is a valid BlockId
		assert.NoError(t, err)
	})

	t.Run("CASE: Maximum number of parents (only strong)", func(t *testing.T) {
		// max number of parents supplied (only strong)
		strongParents := randomParents(MaxParentsCount)
		_, err := NewBlockWithValidation(
			emptyLikeReferencesFromStrongParents(strongParents),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)
	})

	t.Run("CASE: Maximum number of weak parents (one strong)", func(t *testing.T) {
		// max number of weak parents plus one strong
		weakParents := randomParents(MaxParentsCount)
		_, err := NewBlockWithValidation(
			ParentBlockIDs{
				StrongParentType: {EmptyBlockID: types.Void},
				WeakParentType:   weakParents,
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)
	})

	t.Run("CASE: Too many parents, but okay without duplicates", func(t *testing.T) {
		strongParents := randomParents(MaxParentsCount).Slice()
		// MaxParentsCount + 1 parents, but there is one duplicate
		strongParents = append(strongParents, strongParents[MaxParentsCount-1])
		_, err := NewBlockWithValidation(
			emptyLikeReferencesFromStrongParents(NewBlockIDs(strongParents...)),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)
	})
}

func TestBlock_Bytes(t *testing.T) {
	t.Run("CASE: Parents not sorted", func(t *testing.T) {
		blk, err := NewBlockWithValidation(
			ParentBlockIDs{
				StrongParentType: randomParents(4),
				WeakParentType:   randomParents(4),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)

		blkBytes := lo.PanicOnErr(blk.Bytes())
		// bytes 4 to 260 hold the 8 parent IDs
		// manually change their order
		tmp := make([]byte, 32)
		copy(tmp, blkBytes[3:35])
		copy(blkBytes[3:35], blkBytes[3+32:35+32])
		copy(blkBytes[3+32:35+32], tmp)
		err = new(Block).FromBytes(blkBytes)
		assert.Error(t, err)
	})

	t.Run("CASE: Max blk size", func(t *testing.T) {
		// 4 bytes for payload size field + 4 bytes for to denote
		data := make([]byte, payload.MaxSize-8)
		blk, err := NewBlockWithValidation(
			ParentBlockIDs{
				StrongParentType:      randomParents(MaxParentsCount),
				WeakParentType:        randomParents(MaxParentsCount),
				ShallowLikeParentType: randomParents(MaxParentsCount),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(data),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)

		blkBytes := lo.PanicOnErr(blk.Bytes())
		assert.Equal(t, MaxBlockSize, len(blkBytes))
	})

	t.Run("CASE: Min blk size", func(t *testing.T) {
		// blk with minimum number of parents
		blk, err := NewBlockWithValidation(
			ParentBlockIDs{
				StrongParentType: randomParents(MinParentsCount),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(nil),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)

		blkBytes := lo.PanicOnErr(blk.Bytes())
		// 4 full parents blocks - 1 parent block with 1 parent
		assert.Equal(t, MaxBlockSize-payload.MaxSize+8-(2*(1+1+8*BlockIDLength)+(7*BlockIDLength)), len(blkBytes))
	})
}

func TestBlockFromBytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		blk, err := NewBlockWithValidation(
			ParentBlockIDs{
				StrongParentType: randomParents(MaxParentsCount / 2),
				WeakParentType:   randomParents(MaxParentsCount / 2),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("This is a test block.")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err)
		assert.NoError(t, blk.DetermineID())

		blkBytes := lo.PanicOnErr(blk.Bytes())
		result := new(Block)
		err = result.FromBytes(blkBytes)
		assert.NoError(t, result.DetermineID())

		assert.NoError(t, err)
		assert.Equal(t, blk.Version(), result.Version())
		assert.Equal(t, blk.ParentsByType(StrongParentType), result.ParentsByType(StrongParentType))
		assert.Equal(t, blk.ParentsByType(WeakParentType), result.ParentsByType(WeakParentType))
		assert.ElementsMatch(t, blk.Parents(), result.Parents())
		assert.Equal(t, blk.IssuerPublicKey(), result.IssuerPublicKey())
		// time is in different representation but it denotes the same time
		assert.True(t, blk.IssuingTime().Equal(result.IssuingTime()))
		assert.Equal(t, blk.SequenceNumber(), result.SequenceNumber())
		assert.Equal(t, blk.Payload(), result.Payload())
		assert.Equal(t, blk.Nonce(), result.Nonce())
		assert.Equal(t, blk.Signature(), result.Signature())
		assert.Equal(t, blk.ID(), result.ID())
	})

	t.Run("CASE: Trailing bytes", func(t *testing.T) {
		blk, err := NewBlockWithValidation(
			ParentBlockIDs{
				StrongParentType: randomParents(MaxParentsCount / 2),
				WeakParentType:   randomParents(MaxParentsCount / 2),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("This is a test block.")),
			0,
			ed25519.Signature{}, 0, epoch.NewECRecord(0),
		)
		assert.NoError(t, err, "Syntactically invalid block created")
		blkBytes := lo.PanicOnErr(blk.Bytes())
		// put some bytes at the end
		blkBytes = append(blkBytes, []byte{0, 1, 2, 3, 4}...)
		err = new(Block).FromBytes(blkBytes)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, cerrors.ErrParseBytesFailed))
	})
}

func randomTransaction() *devnetvm.Transaction {
	ID, _ := identity.RandomIDInsecure()
	input := devnetvm.NewUTXOInput(utxo.EmptyOutputID)
	var outputs devnetvm.Outputs
	seed := ed25519.NewSeed()
	w := wl{
		keyPair: *seed.KeyPair(0),
		address: devnetvm.NewED25519Address(seed.KeyPair(0).PublicKey),
	}
	output := devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		devnetvm.ColorIOTA: uint64(100),
	}), w.address)
	outputs = append(outputs, output)
	essence := devnetvm.NewTransactionEssence(0, time.Now(), ID, ID, devnetvm.NewInputs(input), outputs)

	unlockBlock := devnetvm.NewSignatureUnlockBlock(w.sign(essence))

	return devnetvm.NewTransaction(essence, devnetvm.UnlockBlocks{unlockBlock})
}

type wl struct {
	keyPair ed25519.KeyPair
	address *devnetvm.ED25519Address
}

func (w wl) privateKey() ed25519.PrivateKey {
	return w.keyPair.PrivateKey
}

func (w wl) publicKey() ed25519.PublicKey {
	return w.keyPair.PublicKey
}

func (w wl) sign(txEssence *devnetvm.TransactionEssence) *devnetvm.ED25519Signature {
	return devnetvm.NewED25519Signature(w.publicKey(), w.privateKey().Sign(lo.PanicOnErr(txEssence.Bytes())))
}
