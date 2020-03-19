package drng

import (
	"testing"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/key"
	"github.com/drand/kyber/share"
	"github.com/drand/kyber/util/random"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/collectiveBeacon"
	"github.com/stretchr/testify/require"
)

func TestVerifyCollectiveBeacon(t *testing.T) {
	payload := dkgShares(t, 5, 3)
	err := VerifyCollectiveBeacon(payload)
	require.NoError(t, err)
}

func TestGetRandomness(t *testing.T) {
	payload := dkgShares(t, 5, 3)
	_, err := GetRandomness(payload.Signature())
	require.NoError(t, err)
}

func dkgShares(t *testing.T, n, threshold int) *collectiveBeacon.Payload {
	var priPoly *share.PriPoly
	var pubPoly *share.PubPoly
	var err error
	// create shares and committments
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
	sigs := make([][]byte, n, n)
	_, commits := pubPoly.Info()
	dkgShares := make([]*key.Share, n, n)

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

	msg = beacon.Message(sig, 1)
	sigs = make([][]byte, n, n)
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

	return collectiveBeacon.New(1, 1, sig, newSig, dpk)
}
