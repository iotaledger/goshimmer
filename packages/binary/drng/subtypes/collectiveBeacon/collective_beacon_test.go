package collectiveBeacon

import (
	"encoding/hex"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
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
	prevSignatureTest, _ = hex.DecodeString("ae9ba6d1445bffea8e66cb7d28fe5924e0a8d31b11b62a8710204e56e1ba84bc3694a3033e5793fcee6e75e956e5da3016cd0e22aa46fa419cd06343a7ff9d1e9c5c08f660f0bdec099e97ef99f470bb8c607ce9667a165e9caa474710f62ffd")
	signatureTest, _ = hex.DecodeString("8dee56fae60dcad960f7176d0813d5415b930cf6e20c299ec2c2dfc5f2ad4903916fd462ba1abf5c32a5bfd94dcc8eba062d011a548d99df7fa1e3bbbc9a0455663d60f6ccc736c1d5b6de727dbe4427e21fb660925518be386265913f447c94")
	dpkTest, _ = hex.DecodeString("a02fcd15edd52c8e134027491a43b597505b466d1679e88f70f927e57c45a93ae0765ff02fc2d015e3a02fd8748e2103")
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
