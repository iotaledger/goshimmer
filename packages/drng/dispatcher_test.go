package drng

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/clock"
)

var (
	prevSignatureTest []byte
	signatureTest     []byte
	dpkTest           []byte
	issuerPK          ed25519.PublicKey
	committeeTest     *Committee
	timestampTest     time.Time
	randomnessTest    *Randomness
)

func init() {
	prevSignatureTest, _ = hex.DecodeString("962c0f195e8a4b281d73952aed13b754e8d0e6be1e0fd0ab0eae76db8cf038d3ec7c82c0f7348f124c2e56df11c7283012758bda8fed44d8fa26ad69781e5853b9b187db878dedd84903584fb168f1287741fae29fe9a4b76a267ae7e0812072")
	signatureTest, _ = hex.DecodeString("94ff0de5d59c87d73e75baf87b084096e4044036bf33c23357c0d5947d3dc876f87a260ce2a53243cd6e627b4771cbdc12c5751b70e885d533831f2b9e83df242dceee54f466537e75fdb7870622345b136c7f5944f84b1278fe83f6d5311d6b")
	dpkTest, _ = hex.DecodeString("80b319dbf164d852cdac3d86f0b362e0131ddeae3d87f6c3c5e3b6a9de384093b983db88f70e2008b0e945657d5980e2")
	timestampTest = clock.SyncedTime()

	rand, _ := ExtractRandomness(signatureTest)
	randomnessTest = &Randomness{
		Round:      1,
		Randomness: rand,
		Timestamp:  timestampTest,
	}

	kp := ed25519.GenerateKeyPair()
	issuerPK = kp.PublicKey

	committeeTest = &Committee{
		InstanceID:    1,
		Threshold:     3,
		Identities:    []ed25519.PublicKey{issuerPK},
		DistributedPK: dpkTest,
	}
}

func testPayload() *CollectiveBeaconPayload {
	header := NewHeader(TypeCollectiveBeacon, 1)
	return NewCollectiveBeaconPayload(header.InstanceID,
		1,
		prevSignatureTest,
		signatureTest,
		dpkTest)
}

func TestDispatcher(t *testing.T) {
	marshalUtil := marshalutil.New(testPayload().Bytes())
	parsedPayload, err := CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	require.NoError(t, err)
	config := make(map[uint32][]Option)
	config[1] = []Option{SetCommittee(committeeTest)}

	drng := New(config)
	err = drng.Dispatch(issuerPK, timestampTest, parsedPayload)
	require.NoError(t, err)
	require.Equal(t, *randomnessTest, drng.State[1].Randomness())
}

func TestEmptyState(t *testing.T) {
	marshalUtil := marshalutil.New(testPayload().Bytes())
	parsedPayload, err := CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	require.NoError(t, err)
	config := make(map[uint32][]Option)

	drng := New(config)
	err = drng.Dispatch(issuerPK, timestampTest, parsedPayload)
	if assert.Error(t, err) {
		assert.Equal(t, ErrInstanceIDMismatch, err)
	}
}
