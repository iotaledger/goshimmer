package collectiveBeacon

import (
	"encoding/hex"
	"log"
	"testing"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/key"
	"github.com/drand/kyber/share"
	"github.com/drand/kyber/util/random"
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

var (
	eventTest         *events.CollectiveBeaconEvent
	prevSignatureTest []byte
	signatureTest     []byte
	dpkTest           []byte
	issuerPK          ed25519.PublicKey
	stateTest         *state.State
)

func init() {
	prevSignatureTest, _ = hex.DecodeString("962c0f195e8a4b281d73952aed13b754e8d0e6be1e0fd0ab0eae76db8cf038d3ec7c82c0f7348f124c2e56df11c7283012758bda8fed44d8fa26ad69781e5853b9b187db878dedd84903584fb168f1287741fae29fe9a4b76a267ae7e0812072")
	signatureTest, _ = hex.DecodeString("94ff0de5d59c87d73e75baf87b084096e4044036bf33c23357c0d5947d3dc876f87a260ce2a53243cd6e627b4771cbdc12c5751b70e885d533831f2b9e83df242dceee54f466537e75fdb7870622345b136c7f5944f84b1278fe83f6d5311d6b")
	dpkTest, _ = hex.DecodeString("80b319dbf164d852cdac3d86f0b362e0131ddeae3d87f6c3c5e3b6a9de384093b983db88f70e2008b0e945657d5980e2")

	kp := ed25519.GenerateKeyPair()
	issuerPK = kp.PublicKey

	eventTest = &events.CollectiveBeaconEvent{
		IssuerPublicKey: issuerPK,
		InstanceID:      1,
		Round:           1,
		PrevSignature:   prevSignatureTest,
		Signature:       signatureTest,
		Dpk:             dpkTest,
	}

	stateTest = state.New(state.SetCommittee(
		&state.Committee{
			InstanceID:    1,
			Threshold:     3,
			Identities:    []ed25519.PublicKey{issuerPK},
			DistributedPK: dpkTest,
		}))
}

func TestVerifySignature(t *testing.T) {
	err := verifySignature(eventTest)
	require.NoError(t, err)
}

func TestGetRandomness(t *testing.T) {
	_, err := ExtractRandomness(eventTest.Signature)
	require.NoError(t, err)
}

func TestProcessBeacon(t *testing.T) {
	err := ProcessBeacon(stateTest, eventTest)
	require.NoError(t, err)
}

func TestDkgShares(t *testing.T) {
	dkgShares(t, 5, 3)
}

func dkgShares(t *testing.T, n, threshold int) *payload.Payload {
	var priPoly *share.PriPoly
	var pubPoly *share.PubPoly
	var err error
	// create shares and commitments
	for i := 0; i < n; i++ {
		pri := share.NewPriPoly(key.KeyGroup, threshold, key.KeyGroup.Scalar().Pick(random.New()), random.New())
		pub := pri.Commit(key.KeyGroup.Point().Base())
		if priPoly == nil {
			priPoly = pri
			pubPoly = pub
			continue
		}
		priPoly, err = priPoly.Add(pri)
		require.NoError(t, err)

		pubPoly, err = pubPoly.Add(pub)
		require.NoError(t, err)
	}
	shares := priPoly.Shares(n)
	secret, err := share.RecoverSecret(key.KeyGroup, shares, threshold, n)
	require.NoError(t, err)
	require.True(t, secret.Equal(priPoly.Secret()))

	msg := []byte("first message")
	sigs := make([][]byte, n)
	_, commits := pubPoly.Info()
	dkgShares := make([]*key.Share, n)

	// partial signatures
	for i := 0; i < n; i++ {
		sigs[i], err = key.Scheme.Sign(shares[i], msg)
		require.NoError(t, err)

		dkgShares[i] = &key.Share{
			Share:   shares[i],
			Commits: commits,
		}
	}

	// reconstruct collective signature
	sig, err := key.Scheme.Recover(pubPoly, msg, sigs, threshold, n)
	require.NoError(t, err)

	// verify signature against distributed public key
	err = key.Scheme.VerifyRecovered(pubPoly.Commit(), msg, sig)
	require.NoError(t, err)

	msg = beacon.Message(1, sig)
	sigs = make([][]byte, n)
	// partial signatures
	for i := 0; i < n; i++ {
		sigs[i], err = key.Scheme.Sign(shares[i], msg)
		require.NoError(t, err)
	}

	// reconstruct collective signature
	newSig, err := key.Scheme.Recover(pubPoly, msg, sigs, threshold, n)
	require.NoError(t, err)

	dpk, err := pubPoly.Commit().MarshalBinary()
	require.NoError(t, err)

	log.Println(hex.EncodeToString(sig))
	log.Println(hex.EncodeToString(newSig))
	log.Println(hex.EncodeToString(dpk))

	return payload.New(1, 1, sig, newSig, dpk)
}
