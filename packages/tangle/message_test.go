package tangle

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
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
	msgBytes := randomBytes(MessageIDLength)
	result, _, _ := MessageIDFromBytes(msgBytes)
	return result
}

func randomParents(count int) []MessageID {
	parents := make([]MessageID, 0, count)
	for i := 0; i < count; i++ {
		parents = append(parents, randomMessageID())
	}
	return parents
}

func testAreParentsSorted(parents []MessageID) bool {
	return sort.SliceIsSorted(parents, func(i, j int) bool {
		return bytes.Compare(parents[i].Bytes(), parents[j].Bytes()) < 0
	})
}

func testSortParents(parents []MessageID) {
	sort.Slice(parents, func(i, j int) bool {
		return bytes.Compare(parents[i].Bytes(), parents[j].Bytes()) < 0
	})
}

func TestNewMessageID(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		randID := randomMessageID()
		randIDString := randID.String()

		result, err := NewMessageID(randIDString)
		assert.NoError(t, err)
		assert.Equal(t, randID, result)
	})

	t.Run("CASE: Not base58 encoded", func(t *testing.T) {
		result, err := NewMessageID("O0l")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to decode base58 encoded string"))
		assert.Equal(t, EmptyMessageID, result)
	})

	t.Run("CASE: Too long string", func(t *testing.T) {
		randID := randomMessageID()
		randIDString := randID.String()

		result, err := NewMessageID(randIDString + "1")
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "length of base58 formatted message id is wrong"))
		assert.Equal(t, EmptyMessageID, result)
	})
}

func TestMessageIDFromBytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		buffer := randomBytes(MessageIDLength)
		result, consumed, err := MessageIDFromBytes(buffer)
		assert.NoError(t, err)
		assert.Equal(t, MessageIDLength, consumed)
		assert.Equal(t, result.Bytes(), buffer)
	})

	t.Run("CASE: Too few bytes", func(t *testing.T) {
		buffer := randomBytes(MessageIDLength - 1)
		result, consumed, err := MessageIDFromBytes(buffer)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "bytes not long enough"))
		assert.Equal(t, 0, consumed)
		assert.Equal(t, EmptyMessageID, result)
	})

	t.Run("CASE: More bytes", func(t *testing.T) {
		buffer := randomBytes(MessageIDLength + 1)
		result, consumed, err := MessageIDFromBytes(buffer)
		assert.NoError(t, err)
		assert.Equal(t, MessageIDLength, consumed)
		assert.Equal(t, buffer[:32], result.Bytes())
	})
}

func TestMessageIDFromMarshalUtil(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		randID := randomMessageID()
		marshalUtil := marshalutil.New(randID.Bytes())
		result, err := MessageIDFromMarshalUtil(marshalUtil)
		assert.NoError(t, err)
		assert.Equal(t, randID, result)
	})

	t.Run("CASE: Wrong bytes in marshalutil", func(t *testing.T) {
		marshalUtil := marshalutil.New(randomBytes(MessageIDLength - 1))
		result, err := MessageIDFromMarshalUtil(marshalUtil)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse message ID"))
		assert.Equal(t, EmptyMessageID, result)
	})
}

func TestMessageID_MarshalBinary(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		randID := randomMessageID()
		result, err := randID.MarshalBinary()
		assert.NoError(t, err)
		assert.Equal(t, randID.Bytes(), result)
	})
}

func TestMessageID_UnmarshalBinary(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		randID1 := randomMessageID()
		randID2 := randomMessageID()
		err := randID1.UnmarshalBinary(randID2.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, randID1, randID2)
	})

	t.Run("CASE: Wrong length (less)", func(t *testing.T) {
		randID := randomMessageID()
		originalBytes := randID.Bytes()
		err := randID.UnmarshalBinary(randomBytes(MessageIDLength - 1))
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), fmt.Sprintf("data must be exactly %d long to encode a valid message id", MessageIDLength)))
		assert.Equal(t, originalBytes, randID.Bytes())
	})

	t.Run("CASE: Wrong length (more)", func(t *testing.T) {
		randID := randomMessageID()
		originalBytes := randID.Bytes()
		err := randID.UnmarshalBinary(randomBytes(MessageIDLength + 1))
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), fmt.Sprintf("data must be exactly %d long to encode a valid message id", MessageIDLength)))
		assert.Equal(t, originalBytes, randID.Bytes())
	})
}

func TestMessageID_String(t *testing.T) {
	randID := randomMessageID()
	randIDString := randID.String()
	assert.Equal(t, base58.Encode(randID.Bytes()), randIDString)
}

func TestMessage_VerifySignature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte("test"))

	unsigned := NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.False(t, unsigned.VerifySignature())

	unsignedBytes := unsigned.Bytes()
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, keyPair.PublicKey, 0, pl, 0, signature)
	assert.True(t, signed.VerifySignature())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()

	testMessage, err := tangle.MessageFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
	require.NoError(t, err)
	assert.Equal(t, true, testMessage.VerifySignature())

	t.Log(testMessage)

	restoredMessage, _, err := MessageFromBytes(testMessage.Bytes())
	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.ID(), restoredMessage.ID())
		assert.ElementsMatch(t, testMessage.StrongParents(), restoredMessage.StrongParents())
		assert.ElementsMatch(t, testMessage.WeakParents(), restoredMessage.WeakParents())
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
		assert.Equal(t, testMessage.IssuingTime().Round(time.Second), restoredMessage.IssuingTime().Round(time.Second))
		assert.Equal(t, testMessage.SequenceNumber(), restoredMessage.SequenceNumber())
		assert.Equal(t, testMessage.Nonce(), restoredMessage.Nonce())
		assert.Equal(t, testMessage.Signature(), restoredMessage.Signature())
		assert.Equal(t, true, restoredMessage.VerifySignature())
	}
}

func TestMessage_NewMessage(t *testing.T) {
	t.Run("CASE: Too many strong parents", func(t *testing.T) {
		// too many strong parents
		strongParents := randomParents(MaxParentsCount + 1)

		assert.Panics(t, func() {
			NewMessage(
				strongParents,
				nil,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Strong and weak parents are individually okay, but, their sum is bigger", func(t *testing.T) {
		// strong and weak parents are individually okay, but, their sum is bigger
		strongParents := randomParents(MaxParentsCount - 1)
		weakParents := randomParents(2)
		assert.Panics(t, func() {
			NewMessage(
				strongParents,
				weakParents,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Too few strong parents", func(t *testing.T) {
		weakParents := randomParents(MaxParentsCount)

		assert.Panics(t, func() {
			NewMessage(
				nil,
				weakParents,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: No parents at all", func(t *testing.T) {
		assert.Panics(t, func() {
			NewMessage(
				nil,
				nil,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Only one weak parent", func(t *testing.T) {
		assert.Panics(t, func() {
			NewMessage(
				nil,
				[]MessageID{EmptyMessageID},
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Minimum number of parents", func(t *testing.T) {
		assert.NotPanics(t, func() {
			NewMessage(
				[]MessageID{EmptyMessageID},
				nil,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Maximum number of parents (only strong)", func(t *testing.T) {
		// max number of parents supplied (only strong)
		strongParents := randomParents(MaxParentsCount)
		assert.NotPanics(t, func() {
			NewMessage(
				strongParents,
				nil,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Maximum number of parents (one strong)", func(t *testing.T) {
		// max number of parents supplied (one strong)
		weakParents := randomParents(MaxParentsCount - 1)
		assert.NotPanics(t, func() {
			NewMessage(
				[]MessageID{EmptyMessageID},
				weakParents,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Too many parents, but okay without duplicates", func(t *testing.T) {
		strongParents := randomParents(MaxParentsCount)
		// MaxParentsCount + 1 parents, but there is one duplicate
		strongParents = append(strongParents, strongParents[MaxParentsCount-1])
		assert.NotPanics(t, func() {
			_ = NewMessage(
				strongParents,
				nil,
				time.Now(),
				ed25519.PublicKey{},
				0,
				payload.NewGenericDataPayload([]byte("")),
				0,
				ed25519.Signature{},
			)
		})
	})

	t.Run("CASE: Strong parents are sorted", func(t *testing.T) {
		// max number of parents supplied (only strong)
		strongParents := randomParents(MaxParentsCount)
		if !testAreParentsSorted(strongParents) {
			testSortParents(strongParents)
			assert.True(t, testAreParentsSorted(strongParents))
		}

		msg := NewMessage(
			strongParents,
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)

		msgStrongParents := msg.StrongParents()
		assert.True(t, testAreParentsSorted(msgStrongParents))
	})

	t.Run("CASE: Weak parents are sorted", func(t *testing.T) {
		// max number of weak parents supplied (MaxParentsCount-1)
		weakParents := randomParents(MaxParentsCount - 1)
		if !testAreParentsSorted(weakParents) {
			testSortParents(weakParents)
			assert.True(t, testAreParentsSorted(weakParents))
		}

		msg := NewMessage(
			[]MessageID{EmptyMessageID},
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)

		msgWeakParents := msg.WeakParents()
		assert.True(t, testAreParentsSorted(msgWeakParents))
	})

	t.Run("CASE: Duplicate strong parents", func(t *testing.T) {
		// max number of parents supplied (only strong)
		strongParents := randomParents(MaxParentsCount / 2)

		strongParents = append(strongParents, strongParents...)

		msg := NewMessage(
			strongParents,
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)

		msgStrongParents := msg.StrongParents()
		assert.True(t, testAreParentsSorted(msgStrongParents))
		assert.Equal(t, MaxParentsCount/2, len(msgStrongParents))
	})

	t.Run("CASE: Duplicate weak parents", func(t *testing.T) {
		weakParents := randomParents(3)
		weakParents = append(weakParents, weakParents...)
		msg := NewMessage(
			[]MessageID{EmptyMessageID},
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)

		msgWeakParents := msg.WeakParents()
		assert.True(t, testAreParentsSorted(msgWeakParents))
		assert.Equal(t, 3, len(msgWeakParents))
	})
}

func TestMessage_Bytes(t *testing.T) {
	t.Run("CASE: Parents sorted", func(t *testing.T) {
		msg := NewMessage(
			randomParents(4),
			randomParents(4),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)

		msgBytes := msg.Bytes()
		_, _, err := MessageFromBytes(msgBytes)
		assert.NoError(t, err)
	})

	t.Run("CASE: Parents not sorted", func(t *testing.T) {
		msg := NewMessage(
			randomParents(4),
			randomParents(4),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)

		msgBytes := msg.Bytes()
		// bytes 4 to 260 hold the 8 parent IDs
		// manually change their order
		tmp := make([]byte, 32)
		copy(tmp, msgBytes[3:35])
		copy(msgBytes[3:35], msgBytes[3+32:35+32])
		copy(msgBytes[3+32:35+32], tmp)
		_, _, err := MessageFromBytes(msgBytes)
		assert.Error(t, err)
	})

	t.Run("CASE: Only strong parents", func(t *testing.T) {
		msg := NewMessage(
			randomParents(MaxParentsCount),
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
		msgBytes := msg.Bytes()

		// byte 2 is the parents count, that should be MaxParentsCount
		assert.Equal(t, MaxParentsCount, int(msgBytes[1]))
		// byte 3 is the parent bitmask. there are only strong parents, so all bits are set in the bitMask
		assert.Equal(t, math.MaxUint8, int(msgBytes[2]))

		strongParents := msg.StrongParents()
		for index, parent := range strongParents {
			parentBytes := parent.Bytes()
			msgParentBytes := msgBytes[3+index*32 : 35+index*32]
			assert.Equal(t, parentBytes, msgParentBytes)
		}
	})

	t.Run("CASE: Strong and weak parents", func(t *testing.T) {
		strongCount := MaxParentsCount / 2
		weakCount := MaxParentsCount / 2
		msg := NewMessage(
			randomParents(strongCount),
			randomParents(weakCount),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
		msgBytes := msg.Bytes()

		// byte 2 is the parents count, that should be MaxParentsCount
		assert.Equal(t, MaxParentsCount, int(msgBytes[1]))
		// byte 3 is the parent bitmask. count the number of set bits
		assert.Equal(t, strongCount, bits.OnesCount8(msgBytes[2]))

		bitMask := bitmask.BitMask(msgBytes[2])
		bytesStrongParents := make([][]byte, 0)
		bytesWeakParents := make([][]byte, 0)
		bothParentsFromBytes := make([]MessageID, MaxParentsCount)

		//extract raw bytes of the parents
		for i := 0; i < MaxParentsCount; i++ {
			parentBytes := make([]byte, 32)
			copy(parentBytes, msgBytes[3+i*32:35+i*32])
			if bitMask.HasBit(uint(i)) {
				bytesStrongParents = append(bytesStrongParents, parentBytes)
			} else {
				bytesWeakParents = append(bytesWeakParents, parentBytes)
			}
			// build combined parent list for later checks
			parsedMsgID, _, err := MessageIDFromBytes(parentBytes)
			assert.NoError(t, err)
			bothParentsFromBytes = append(bothParentsFromBytes, parsedMsgID)
		}
		// StrongParents() returns sorted strong parents. We compare it to bytesStrongParents, that was filled in the
		// order of appearance.
		for index, parent := range msg.StrongParents() {
			assert.Equal(t, parent.Bytes(), bytesStrongParents[index])
		}
		// WeakParents() returns sorted weak parents. We compare it to bytesWeakParents, that was filled in the
		// order of appearance.
		for index, parent := range msg.WeakParents() {
			assert.Equal(t, parent.Bytes(), bytesWeakParents[index])
		}

		// test that parents were sorted in the msg bytes
		assert.True(t, testAreParentsSorted(bothParentsFromBytes))
	})

	t.Run("CASE: Max msg size", func(t *testing.T) {
		// 4 + 4 bytes for payload header
		data := make([]byte, payload.MaxSize-4-4)
		msg := NewMessage(
			randomParents(MaxParentsCount),
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(data),
			0,
			ed25519.Signature{},
		)

		msgBytes := msg.Bytes()
		assert.Equal(t, MaxMessageSize, len(msgBytes))
	})

	t.Run("CASE: Min msg sixe", func(t *testing.T) {
		// msg with maxmimum number of parents
		msg := NewMessage(
			randomParents(MaxParentsCount),
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(nil),
			0,
			ed25519.Signature{},
		)

		msgBytes := msg.Bytes()
		// 4 + 4 bytes for payload header
		assert.Equal(t, MaxMessageSize-payload.MaxSize+4+4, len(msgBytes))

		// msg with minimum number of parents
		msg2 := NewMessage(
			randomParents(MinParentsCount),
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload(nil),
			0,
			ed25519.Signature{},
		)

		msgBytes2 := msg2.Bytes()
		// 4 + 4 bytes for payload header
		assert.Equal(t, MaxMessageSize-payload.MaxSize+4+4-32*(MaxParentsCount-MinParentsCount), len(msgBytes2))
	})
}

func TestMessageFromBytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		msg := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{},
		)
		msgBytes := msg.Bytes()
		result, consumedBytes, err := MessageFromBytes(msgBytes)
		assert.Equal(t, len(msgBytes), consumedBytes)
		assert.NoError(t, err)
		assert.Equal(t, msg.Version(), result.Version())
		assert.Equal(t, msg.StrongParents(), result.StrongParents())
		assert.Equal(t, msg.WeakParents(), result.WeakParents())
		assert.Equal(t, msg.ParentsCount(), result.ParentsCount())
		assert.Equal(t, msg.IssuerPublicKey(), result.IssuerPublicKey())
		// time is in different representation but it denotes the same time
		assert.True(t, msg.IssuingTime().Equal(result.IssuingTime()))
		assert.Equal(t, msg.SequenceNumber(), result.SequenceNumber())
		assert.Equal(t, msg.Payload(), result.Payload())
		assert.Equal(t, msg.Nonce(), result.Nonce())
		assert.Equal(t, msg.Signature(), result.Signature())
		assert.Equal(t, msg.calculateID(), result.calculateID())
	})

	t.Run("CASE: Trailing bytes", func(t *testing.T) {
		msg := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{},
		)
		msgBytes := msg.Bytes()
		// put some bytes at the end
		msgBytes = append(msgBytes, []byte{0, 1, 2, 3, 4}...)
		_, _, err := MessageFromBytes(msgBytes)
		assert.Error(t, err)
		assert.True(t, xerrors.Is(err, cerrors.ErrParseBytesFailed))
	})

}

func createTestMsgBytes(numStrongParents int, numWeakParents int) []byte {
	msg := NewMessage(
		randomParents(numStrongParents),
		randomParents(numWeakParents),
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("This is a test message.")),
		0,
		ed25519.Signature{},
	)

	return msg.Bytes()
}

func TestMessageFromMarshalUtil(t *testing.T) {
	t.Run("CASE: Missing version", func(t *testing.T) {
		marshaller := marshalutil.New([]byte{})
		// missing version
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse message version"))
		assert.Equal(t, uint8(0), result.Version())
	})

	t.Run("CASE: Missing parents count", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		// missing parentsCount
		marshaller := marshalutil.New(msgBytes[:1])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse parents count"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(0), result.ParentsCount())
	})

	t.Run("CASE: Invalid parents count (less)", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		msgBytes[1] = MinParentsCount - 1
		marshaller := marshalutil.New(msgBytes[:2])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), fmt.Sprintf("parents count %d not allowed", MinParentsCount-1)))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(0), result.ParentsCount())
	})

	t.Run("CASE: Invalid parents count (more)", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		msgBytes[1] = MaxParentsCount + 1
		marshaller := marshalutil.New(msgBytes[:2])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), fmt.Sprintf("parents count %d not allowed", MaxParentsCount+1)))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(0), result.ParentsCount())
	})

	t.Run("CASE: Missing parent types", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		marshaller := marshalutil.New(msgBytes[:2])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse parent types"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(0), result.ParentsCount())
	})

	t.Run("CASE: Invalid parent types", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		msgBytes[2] = 0
		marshaller := marshalutil.New(msgBytes[:3])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid parent types, no strong parent specified"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(0), result.ParentsCount())
	})

	t.Run("CASE: Missing parents (all)", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		marshaller := marshalutil.New(msgBytes[:3])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse parent"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(0), result.ParentsCount())
	})

	t.Run("CASE: Missing parents (one)", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		marshaller := marshalutil.New(msgBytes[:3+(MaxParentsCount-1)*32])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse parent"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(MaxParentsCount-1), result.ParentsCount())
	})

	t.Run("CASE: Unsorted parents", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		// put and empty msg ID at position MinParentsCount+1
		copy(msgBytes[3+MinParentsCount*32:3+(MinParentsCount+1)*32], EmptyMessageID.Bytes())
		marshaller := marshalutil.New(msgBytes[:3+(MinParentsCount+1)*32])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "parents not sorted lexicographically ascending"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(MinParentsCount+1), result.ParentsCount())
	})

	t.Run("CASE: Wrong min strong parent count", func(t *testing.T) {
		msgBytes := createTestMsgBytes(2, 0)
		// modify parentTypes bitmask to have at least 1 bit set, but at the wrong position
		msgBytes[2] = 32
		marshaller := marshalutil.New(msgBytes[:3+2*32])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "strong parents count 0 not allowed"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(2), result.ParentsCount())
	})

	t.Run("CASE: Missing issuer public key bytes", func(t *testing.T) {
		msgBytes := createTestMsgBytes(MaxParentsCount/2, MaxParentsCount/2)
		// +8*32 for the 8 parents
		marshaller := marshalutil.New(msgBytes[:3+MaxParentsCount*32])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse issuer public key"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(8), result.ParentsCount())
		assert.Equal(t, 4, len(result.StrongParents()))
		assert.Equal(t, 4, len(result.WeakParents()))
	})

	t.Run("CASE: Missing issuing time bytes", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey()
		msgBytes := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			time.Now(),
			pub,
			0,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{},
		).Bytes()

		// +32 for issuer public key
		marshaller := marshalutil.New(msgBytes[:3+MaxParentsCount*32+32])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse issuing time"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(8), result.ParentsCount())
		assert.Equal(t, 4, len(result.StrongParents()))
		assert.Equal(t, 4, len(result.WeakParents()))
		assert.Equal(t, pub, result.IssuerPublicKey())
	})

	t.Run("CASE: Missing sequence number bytes", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey()
		testTime := time.Now()
		msgBytes := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			testTime,
			pub,
			0,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{},
		).Bytes()

		// +8 for time
		marshaller := marshalutil.New(msgBytes[:3+MaxParentsCount*32+32+8])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse sequence number"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(8), result.ParentsCount())
		assert.Equal(t, 4, len(result.StrongParents()))
		assert.Equal(t, 4, len(result.WeakParents()))
		assert.Equal(t, pub, result.IssuerPublicKey())
		assert.True(t, result.IssuingTime().Equal(testTime))
	})

	t.Run("CASE: Missing payload", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey()
		testTime := time.Now()
		msgBytes := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			testTime,
			pub,
			666,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			0,
			ed25519.Signature{},
		).Bytes()

		// +8 for seq num
		marshaller := marshalutil.New(msgBytes[:3+MaxParentsCount*32+32+8+8])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse payload"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(8), result.ParentsCount())
		assert.Equal(t, 4, len(result.StrongParents()))
		assert.Equal(t, 4, len(result.WeakParents()))
		assert.Equal(t, pub, result.IssuerPublicKey())
		assert.True(t, result.IssuingTime().Equal(testTime))
		assert.Equal(t, uint64(666), result.SequenceNumber())
	})

	t.Run("CASE: Missing nonce bytes", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey()
		testTime := time.Now()
		data := []byte("This is a test message.")
		msgBytes := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			testTime,
			pub,
			666,
			payload.NewGenericDataPayload(data),
			0,
			ed25519.Signature{},
		).Bytes()

		// +8 for payload headers + the payload data length
		marshaller := marshalutil.New(msgBytes[:3+MaxParentsCount*32+32+8+8+8+len(data)])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse nonce"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(8), result.ParentsCount())
		assert.Equal(t, 4, len(result.StrongParents()))
		assert.Equal(t, 4, len(result.WeakParents()))
		assert.Equal(t, pub, result.IssuerPublicKey())
		assert.True(t, result.IssuingTime().Equal(testTime))
		assert.Equal(t, uint64(666), result.SequenceNumber())
		assert.Equal(t, data, result.Payload().(*payload.GenericDataPayload).Blob())
	})

	t.Run("CASE: Missing signature", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey()
		testTime := time.Now()
		data := []byte("This is a test message.")
		msgBytes := NewMessage(
			randomParents(MaxParentsCount/2),
			randomParents(MaxParentsCount/2),
			testTime,
			pub,
			666,
			payload.NewGenericDataPayload(data),
			99,
			ed25519.Signature{},
		).Bytes()

		// +8 for nonce
		marshaller := marshalutil.New(msgBytes[:3+MaxParentsCount*32+32+8+8+8+len(data)+8])
		result, err := MessageFromMarshalUtil(marshaller)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed to parse signature"))
		assert.Equal(t, MessageVersion, result.Version())
		assert.Equal(t, uint8(8), result.ParentsCount())
		assert.Equal(t, 4, len(result.StrongParents()))
		assert.Equal(t, 4, len(result.WeakParents()))
		assert.Equal(t, pub, result.IssuerPublicKey())
		assert.True(t, result.IssuingTime().Equal(testTime))
		assert.Equal(t, uint64(666), result.SequenceNumber())
		assert.Equal(t, data, result.Payload().(*payload.GenericDataPayload).Blob())
		assert.Equal(t, uint64(99), result.Nonce())
	})
}

func TestMessage_ForEachParent(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		strongParents := randomParents(MaxParentsCount / 2)
		weakParents := randomParents(MaxParentsCount / 2)

		msg := NewMessage(
			strongParents,
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			666,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			99,
			ed25519.Signature{},
		)

		sortedStrongParents := sortParents(strongParents)
		sortedWeakParents := sortParents(weakParents)
		sortedStrongWeakParents := append(sortedStrongParents, sortedWeakParents...)
		var resultParents = make([]MessageID, 0)
		checker := func(parent Parent) {
			resultParents = append(resultParents, parent.ID)
		}
		msg.ForEachParent(checker)

		assert.Equal(t, sortedStrongWeakParents, resultParents)
	})
}

func TestMessage_ForEachStrongParent(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		strongParents := randomParents(MaxParentsCount / 2)
		weakParents := randomParents(MaxParentsCount / 2)

		msg := NewMessage(
			strongParents,
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			666,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			99,
			ed25519.Signature{},
		)

		sortedStrongParents := sortParents(strongParents)
		var resultParents = make([]MessageID, 0)
		checker := func(parent MessageID) {
			resultParents = append(resultParents, parent)
		}
		msg.ForEachStrongParent(checker)

		assert.Equal(t, sortedStrongParents, resultParents)
	})
}

func TestMessage_ForEachWeakParent(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		strongParents := randomParents(MaxParentsCount / 2)
		weakParents := randomParents(MaxParentsCount / 2)

		msg := NewMessage(
			strongParents,
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			666,
			payload.NewGenericDataPayload([]byte("This is a test message.")),
			99,
			ed25519.Signature{},
		)

		sortedWeakParents := sortParents(weakParents)
		var resultParents = make([]MessageID, 0)
		checker := func(parent MessageID) {
			resultParents = append(resultParents, parent)
		}
		msg.ForEachWeakParent(checker)

		assert.Equal(t, sortedWeakParents, resultParents)
	})
}
