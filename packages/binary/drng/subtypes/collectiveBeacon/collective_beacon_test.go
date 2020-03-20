package collectiveBeacon

import (
	"encoding/hex"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/stretchr/testify/require"
)

var (
	eventTest         *events.CollectiveBeaconEvent
	prevSignatureTest []byte
	signatureTest     []byte
	dpkTest           []byte
	issuerPK          ed25119.PublicKey
	stateTest         *state.State
)

func init() {
	prevSignatureTest, _ = hex.DecodeString("ae9ba6d1445bffea8e66cb7d28fe5924e0a8d31b11b62a8710204e56e1ba84bc3694a3033e5793fcee6e75e956e5da3016cd0e22aa46fa419cd06343a7ff9d1e9c5c08f660f0bdec099e97ef99f470bb8c607ce9667a165e9caa474710f62ffd")
	signatureTest, _ = hex.DecodeString("8dee56fae60dcad960f7176d0813d5415b930cf6e20c299ec2c2dfc5f2ad4903916fd462ba1abf5c32a5bfd94dcc8eba062d011a548d99df7fa1e3bbbc9a0455663d60f6ccc736c1d5b6de727dbe4427e21fb660925518be386265913f447c94")
	dpkTest, _ = hex.DecodeString("a02fcd15edd52c8e134027491a43b597505b466d1679e88f70f927e57c45a93ae0765ff02fc2d015e3a02fd8748e2103")
	kp := ed25119.GenerateKeyPair()
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
			Identities:    []ed25119.PublicKey{issuerPK},
			DistributedPK: dpkTest,
		}))
}

func TestVerifySignature(t *testing.T) {
	//payload := dkgShares(t, 5, 3)
	err := verifySignature(eventTest)
	require.NoError(t, err)
}

func TestGetRandomness(t *testing.T) {
	//payload := dkgShares(t, 5, 3)
	_, err := GetRandomness(eventTest.Signature)
	require.NoError(t, err)
}

func TestProcessBeacon(t *testing.T) {
	err := ProcessBeacon(stateTest, eventTest)
	require.NoError(t, err)
}

// func dkgShares(t *testing.T, n, threshold int) *collectiveBeacon.Payload {
// 	var priPoly *share.PriPoly
// 	var pubPoly *share.PubPoly
// 	var err error
// 	// create shares and committments
// 	for i := 0; i < n; i++ {
// 		pri := share.NewPriPoly(key.KeyGroup, threshold, key.KeyGroup.Scalar().Pick(random.New()), random.New())
// 		pub := pri.Commit(key.KeyGroup.Point().Base())
// 		if priPoly == nil {
// 			priPoly = pri
// 			pubPoly = pub
// 			continue
// 		}
// 		priPoly, err = priPoly.Add(pri)
// 		require.NoError(t, err)

// 		pubPoly, err = pubPoly.Add(pub)
// 		require.NoError(t, err)
// 	}
// 	shares := priPoly.Shares(n)
// 	secret, err := share.RecoverSecret(key.KeyGroup, shares, threshold, n)
// 	require.NoError(t, err)
// 	require.True(t, secret.Equal(priPoly.Secret()))

// 	msg := []byte("first message")
// 	sigs := make([][]byte, n, n)
// 	_, commits := pubPoly.Info()
// 	dkgShares := make([]*key.Share, n, n)

// 	// partial signatures
// 	for i := 0; i < n; i++ {
// 		sigs[i], err = key.Scheme.Sign(shares[i], msg)
// 		require.NoError(t, err)

// 		dkgShares[i] = &key.Share{
// 			Share:   shares[i],
// 			Commits: commits,
// 		}
// 	}

// 	// reconstruct collective signature
// 	sig, err := key.Scheme.Recover(pubPoly, msg, sigs, threshold, n)
// 	require.NoError(t, err)

// 	// verify signature against distributed public key
// 	err = key.Scheme.VerifyRecovered(pubPoly.Commit(), msg, sig)
// 	require.NoError(t, err)

// 	msg = beacon.Message(sig, 1)
// 	sigs = make([][]byte, n, n)
// 	// partial signatures
// 	for i := 0; i < n; i++ {
// 		sigs[i], err = key.Scheme.Sign(shares[i], msg)
// 		require.NoError(t, err)
// 	}

// 	// reconstruct collective signature
// 	newSig, err := key.Scheme.Recover(pubPoly, msg, sigs, threshold, n)
// 	require.NoError(t, err)

// 	dpk, err := pubPoly.Commit().MarshalBinary()
// 	require.NoError(t, err)

// 	return collectiveBeacon.New(1, 1, sig, newSig, dpk)
// }
