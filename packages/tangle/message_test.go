package tangle

import (
	"bytes"
	"crypto/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func randomBytes(size uint) []byte {
	buffer := make([]byte, size)
	_, err := rand.Read(buffer)
	if err != nil {
		panic(err)
	}
	return buffer
}

func randomMessageID() MessageID {
	var result MessageID
	err := result.FromRandomness()
	if err != nil {
		panic(err)
	}
	return result
}

func randomParents(count int) MessageIDs {
	parents := NewMessageIDs()
	for i := 0; i < count; i++ {
		parents.Add(randomMessageID())
	}
	return parents
}

func testSortParents(parents MessageIDs) []MessageID {
	parentsSorted := parents.Slice()
	sort.Slice(parentsSorted, func(i, j int) bool {
		return bytes.Compare(parentsSorted[i].Bytes(), parentsSorted[j].Bytes()) < 0
	})
	return parentsSorted
}

func TestNewMessageID(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		randID := randomMessageID()
		randIDString := randID.Base58()
		var msgID MessageID
		err := msgID.FromBase58(randIDString)
		assert.NoError(t, err)
		assert.Equal(t, randID, msgID)
	})

	t.Run("CASE: Not base58 encoded", func(t *testing.T) {
		var msgID MessageID
		err := msgID.FromBase58("O0l")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to decode base58 encoded string"))
		assert.Equal(t, EmptyMessageID, msgID)
	})
}

func TestMessageIDFromBytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		buffer := randomBytes(MessageIDLength)
		var msgID MessageID
		consumed, err := msgID.Decode(buffer)
		assert.NoError(t, err)
		assert.Equal(t, MessageIDLength, consumed)
		assert.Equal(t, msgID.Bytes(), buffer)
	})

	t.Run("CASE: Too few bytes", func(t *testing.T) {
		buffer := randomBytes(MessageIDLength - 1)
		var result MessageID
		consumed, err := result.Decode(buffer)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "not enough data to decode Identifier"))
		assert.Equal(t, 0, consumed)
		assert.Equal(t, EmptyMessageID, result)
	})

	t.Run("CASE: More bytes", func(t *testing.T) {
		buffer := randomBytes(MessageIDLength + 1)
		var result MessageID
		consumed, err := result.Decode(buffer)
		assert.NoError(t, err)
		assert.Equal(t, MessageIDLength, consumed)
		assert.Equal(t, buffer[:32], result.Bytes())
	})
}

func TestMessageID_String(t *testing.T) {
	randID := randomMessageID()
	randIDString := randID.String()
	assert.Equal(t, "MessageID("+base58.Encode(randID.Bytes())+")", randIDString)
}

func TestMessageID_Base58(t *testing.T) {
	randID := randomMessageID()
	randIDString := randID.Base58()
	assert.Equal(t, base58.Encode(randID.Bytes()), randIDString)
}

func TestMessage_VerifySignature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte("test"))

	unsigned := NewMessage(NewParentMessageIDs().AddStrong(EmptyMessageID), time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{}, 0, nil)
	assert.False(t, lo.PanicOnErr(unsigned.VerifySignature()))

	unsignedBytes, err := unsigned.Bytes()
	require.NoError(t, err)
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := NewMessage(NewParentMessageIDs().AddStrong(EmptyMessageID), time.Time{}, keyPair.PublicKey, 0, pl, 0, signature, 0, nil)
	assert.True(t, lo.PanicOnErr(signed.VerifySignature()))
}

func TestMessage_UnmarshalTransaction(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	references := ParentMessageIDs{
		StrongParentType: randomParents(1),
		WeakParentType:   randomParents(1),
	}

	testMessage, err := NewMessageWithValidation(references,
		time.Now(),
		ed25519.PublicKey{},
		0,
		randomTransaction(),
		0,
		ed25519.Signature{}, 0, nil)
	assert.NoError(t, err)
	assert.NoError(t, testMessage.DetermineID())

	restoredMessage := new(Message)
	err = restoredMessage.FromBytes(lo.PanicOnErr(testMessage.Bytes()))
	assert.NoError(t, err)
	assert.NoError(t, restoredMessage.DetermineID())
	assert.Equal(t, testMessage.ID(), restoredMessage.ID())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testMessage, err := tangle.MessageFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
	require.NoError(t, err)
	assert.Equal(t, true, lo.PanicOnErr(testMessage.VerifySignature()))

	t.Log(testMessage)

	restoredMessage := new(Message)
	err = restoredMessage.FromBytes(lo.PanicOnErr(testMessage.Bytes()))
	assert.NoError(t, restoredMessage.DetermineID())

	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.ID(), restoredMessage.ID())
		assert.Equal(t, testMessage.ParentsByType(StrongParentType), restoredMessage.ParentsByType(StrongParentType))
		assert.Equal(t, testMessage.ParentsByType(WeakParentType), restoredMessage.ParentsByType(WeakParentType))
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
		assert.Equal(t, testMessage.IssuingTime().Round(time.Second), restoredMessage.IssuingTime().Round(time.Second))
		assert.Equal(t, testMessage.SequenceNumber(), restoredMessage.SequenceNumber())
		assert.Equal(t, testMessage.Nonce(), restoredMessage.Nonce())
		assert.Equal(t, testMessage.Signature(), restoredMessage.Signature())
		assert.Equal(t, true, lo.PanicOnErr(restoredMessage.VerifySignature()))
	}
}

func TestNewMessageWithValidation(t *testing.T) {
	t.Run("CASE: Too many strong parents", func(t *testing.T) {
		// too many strong parents
		strongParents := testSortParents(randomParents(MaxParentsCount + 1))
		parentBlocks := NewParentMessageIDs()
		parentBlocks.AddAll(StrongParentType, NewMessageIDs(strongParents...))

		_, err := NewMessageWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0, nil, MessageVersion)
		assert.Error(t, err)
	})

	t.Run("CASE: Nil block", func(t *testing.T) {
		_, err := NewMessageWithValidation(
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			nil,
			MessageVersion,
		)
		assert.Error(t, err)
	})

	t.Run("CASE: Empty Block", func(t *testing.T) {
		_, err := NewMessageWithValidation(
			NewParentMessageIDs(),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			nil,
			MessageVersion,
		)
		assert.Error(t, err)
	})

	t.Run("CASE: Blocks are unordered", func(t *testing.T) {
		parents := testSortParents(randomParents(1))
		parentBlocks := NewParentMessageIDs()
		parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))
		parentBlocks.AddAll(WeakParentType, NewMessageIDs(testSortParents(randomParents(MaxParentsCount))...))
		parentBlocks.AddAll(ShallowLikeParentType, NewMessageIDs(parents...))

		msg, err := NewMessageWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			nil,
			MessageVersion,
		)
		assert.NoError(t, err)
		assert.NoError(t, msg.DetermineID())
		msgBytes := lo.PanicOnErr(msg.Bytes())

		msgBytes[2] = byte(WeakParentType)
		msgBytes[36] = byte(StrongParentType)

		err = new(Message).FromObjectStorage(msg.IDBytes(), msgBytes)
		assert.ErrorContains(t, err, "array elements must be in their lexical order (byte wise)")
	})

	t.Run("CASE: Repeating block types", func(t *testing.T) {
		// this can be tested only for deserialization as it's even impossible to create such message using current structures

		parents := testSortParents(randomParents(MaxParentsCount))
		parentBlocks := NewParentMessageIDs()
		parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))
		parentBlocks.AddAll(WeakParentType, NewMessageIDs(testSortParents(randomParents(MaxParentsCount))...))
		parentBlocks.AddAll(ShallowLikeParentType, NewMessageIDs(parents...))

		msg, err := NewMessageWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			nil,
			MessageVersion,
		)
		assert.NoError(t, msg.DetermineID())
		msgBytes := lo.PanicOnErr(msg.Bytes())

		msgBytes[2] = byte(WeakParentType)

		err = new(Message).FromObjectStorage(msg.IDBytes(), msgBytes)

		assert.ErrorContains(t, err, "array elements must be unique")
	})

	t.Run("CASE: Unknown block type", func(t *testing.T) {
		parents := testSortParents(randomParents(MaxParentsCount))

		parentBlocks := NewParentMessageIDs()
		parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))
		parentBlocks.AddAll(ShallowLikeParentType, NewMessageIDs(parents...))
		parentBlocks.AddAll(LastValidBlockType+1, NewMessageIDs(parents...))

		_, err := NewMessageWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			nil,
			MessageVersion,
		)
		assert.Error(t, err)
	})

	t.Run("Case: Duplicate references", func(t *testing.T) {
		parents := testSortParents(randomParents(4))

		parentBlocks := NewParentMessageIDs()
		parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))

		msg, err := NewMessageWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0,
			nil,
			MessageVersion,
		)
		assert.NoError(t, msg.DetermineID())
		msgBytes := lo.PanicOnErr(msg.Bytes())

		copy(msgBytes[4:36], msgBytes[36:36+32])

		err = msg.FromObjectStorage(msg.IDBytes(), msgBytes)
		assert.ErrorContains(t, err, "array elements must be unique")
	})
	t.Run("Case: References out of order", func(t *testing.T) {
		parents := testSortParents(randomParents(4))

		parentBlocks := NewParentMessageIDs()
		parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))

		msg, err := NewMessageWithValidation(
			parentBlocks,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
			0, nil,
			MessageVersion,
		)
		assert.NoError(t, msg.DetermineID())

		msgBytes := lo.PanicOnErr(msg.Bytes())

		// replace parents in byte structure
		copy(msgBytes[4:36], msgBytes[36+32:36+64])

		err = msg.FromObjectStorage(msg.IDBytes(), msgBytes)
		assert.ErrorContains(t, err, "array elements must be in their lexical order (byte wise)")
		// if the duplicates are not consecutive a lexicographically order error is returned
	})

	t.Run("Parents Repeating across blocks", func(t *testing.T) {
		parents := testSortParents(randomParents(4))

		{
			parentBlocks := NewParentMessageIDs()
			parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))
			parentBlocks.AddAll(ShallowLikeParentType, NewMessageIDs(parents...))

			_, err := NewMessageWithValidation(
				parentBlocks,
				time.Now(),
				ed25519.PublicKey{},
				0,

				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
				0,
				nil,
				MessageVersion,
			)
			assert.NoError(t, err, "strong and like parents may have duplicate parents")
		}

		{
			parentBlocks := NewParentMessageIDs()
			parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))
			parentBlocks.AddAll(WeakParentType, NewMessageIDs(parents...))

			_, err := NewMessageWithValidation(
				parentBlocks,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
				0,
				nil,
				MessageVersion,
			)
			assert.Error(t, err, "messages in weak references may allow to overlap with strong references")
		}

		{
			// check for repeating message across weak and dislike block
			weakParents := testSortParents(randomParents(4))
			weakParents[2] = parents[2]

			parentBlocks := NewParentMessageIDs()
			parentBlocks.AddAll(StrongParentType, NewMessageIDs(parents...))
			parentBlocks.AddAll(WeakParentType, NewMessageIDs(weakParents...))

			_, err := NewMessageWithValidation(
				parentBlocks,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
				0,
				nil,
				MessageVersion)
			assert.Error(t, err)
		}
	})
}

func TestMessage_NewMessage(t *testing.T) {
	t.Run("CASE: No parents at all", func(t *testing.T) {
		_, err := NewMessageWithValidation(
			ParentMessageIDs{},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.ErrorIs(t, err, ErrNoStrongParents)
	})

	t.Run("CASE: Minimum number of parents", func(t *testing.T) {
		_, err := NewMessageWithValidation(
			emptyLikeReferencesFromStrongParents(NewMessageIDs(EmptyMessageID)),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		// should pass since EmptyMessageId is a valid MessageId
		assert.NoError(t, err)
	})

	t.Run("CASE: Maximum number of parents (only strong)", func(t *testing.T) {
		// max number of parents supplied (only strong)
		strongParents := randomParents(MaxParentsCount)
		_, err := NewMessageWithValidation(
			emptyLikeReferencesFromStrongParents(strongParents),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)
	})

	t.Run("CASE: Maximum number of weak parents (one strong)", func(t *testing.T) {
		// max number of weak parents plus one strong
		weakParents := randomParents(MaxParentsCount)
		_, err := NewMessageWithValidation(
			ParentMessageIDs{
				StrongParentType: {EmptyMessageID: types.Void},
				WeakParentType:   weakParents,
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)
	})

	t.Run("CASE: Too many parents, but okay without duplicates", func(t *testing.T) {
		strongParents := randomParents(MaxParentsCount).Slice()
		// MaxParentsCount + 1 parents, but there is one duplicate
		strongParents = append(strongParents, strongParents[MaxParentsCount-1])
		_, err := NewMessageWithValidation(
			emptyLikeReferencesFromStrongParents(NewMessageIDs(strongParents...)),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)
	})
}

func TestMessage_Bytes(t *testing.T) {
	t.Run("CASE: Parents not sorted", func(t *testing.T) {
		msg, err := NewMessageWithValidation(
			ParentMessageIDs{
				StrongParentType: randomParents(4),
				WeakParentType:   randomParents(4),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)

		msgBytes := lo.PanicOnErr(msg.Bytes())
		// bytes 4 to 260 hold the 8 parent IDs
		// manually change their order
		tmp := make([]byte, 32)
		copy(tmp, msgBytes[3:35])
		copy(msgBytes[3:35], msgBytes[3+32:35+32])
		copy(msgBytes[3+32:35+32], tmp)
		err = new(Message).FromBytes(msgBytes)
		assert.Error(t, err)
	})

	t.Run("CASE: Max msg size", func(t *testing.T) {
		// 4 bytes for payload size field + 4 bytes for to denote
		data := make([]byte, payload.MaxSize-8)
		msg, err := NewMessageWithValidation(
			ParentMessageIDs{
				StrongParentType:      randomParents(MaxParentsCount),
				WeakParentType:        randomParents(MaxParentsCount),
				ShallowLikeParentType: randomParents(MaxParentsCount),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(data),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)

		msgBytes := lo.PanicOnErr(msg.Bytes())
		assert.Equal(t, MaxMessageSize, len(msgBytes))
	})

	t.Run("CASE: Min msg size", func(t *testing.T) {
		// msg with minimum number of parents
		msg, err := NewMessageWithValidation(
			ParentMessageIDs{
				StrongParentType: randomParents(MinParentsCount),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(nil),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)

		msgBytes := lo.PanicOnErr(msg.Bytes())
		// 4 full parents blocks - 1 parent block with 1 parent
		assert.Equal(t, MaxMessageSize-payload.MaxSize+8-(2*(1+1+8*32)+(7*32)), len(msgBytes))
	})
}

func TestMessageFromBytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		msg, err := NewMessageWithValidation(
			ParentMessageIDs{
				StrongParentType: randomParents(MaxParentsCount / 2),
				WeakParentType:   randomParents(MaxParentsCount / 2),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err)
		assert.NoError(t, msg.DetermineID())

		msgBytes := lo.PanicOnErr(msg.Bytes())
		result := new(Message)
		err = result.FromBytes(msgBytes)
		assert.NoError(t, result.DetermineID())

		assert.NoError(t, err)
		assert.Equal(t, msg.Version(), result.Version())
		assert.Equal(t, msg.ParentsByType(StrongParentType), result.ParentsByType(StrongParentType))
		assert.Equal(t, msg.ParentsByType(WeakParentType), result.ParentsByType(WeakParentType))
		assert.ElementsMatch(t, msg.Parents(), result.Parents())
		assert.Equal(t, msg.IssuerPublicKey(), result.IssuerPublicKey())
		// time is in different representation but it denotes the same time
		assert.True(t, msg.IssuingTime().Equal(result.IssuingTime()))
		assert.Equal(t, msg.SequenceNumber(), result.SequenceNumber())
		assert.Equal(t, msg.Payload(), result.Payload())
		assert.Equal(t, msg.Nonce(), result.Nonce())
		assert.Equal(t, msg.Signature(), result.Signature())
		assert.Equal(t, msg.ID(), result.ID())
	})

	t.Run("CASE: Trailing bytes", func(t *testing.T) {
		msg, err := NewMessageWithValidation(
			ParentMessageIDs{
				StrongParentType: randomParents(MaxParentsCount / 2),
				WeakParentType:   randomParents(MaxParentsCount / 2),
			},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{}, 0, nil,
		)
		assert.NoError(t, err, "Syntactically invalid message created")
		msgBytes := lo.PanicOnErr(msg.Bytes())
		// put some bytes at the end
		msgBytes = append(msgBytes, []byte{0, 1, 2, 3, 4}...)
		err = new(Message).FromBytes(msgBytes)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, cerrors.ErrParseBytesFailed))
	})
}

func randomTransaction() *devnetvm.Transaction {
	ID, _ := identity.RandomID()
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
