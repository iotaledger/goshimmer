package drng

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	cbPayload "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/stretchr/testify/require"
)

var (
	eventTest         *events.CollectiveBeaconEvent
	prevSignatureTest []byte
	signatureTest     []byte
	dpkTest           []byte
	issuerPK          ed25119.PublicKey
	committeeTest     *state.Committee
	timestampTest     time.Time
	randomnessTest    *state.Randomness
)

func init() {
	prevSignatureTest, _ = hex.DecodeString("ae9ba6d1445bffea8e66cb7d28fe5924e0a8d31b11b62a8710204e56e1ba84bc3694a3033e5793fcee6e75e956e5da3016cd0e22aa46fa419cd06343a7ff9d1e9c5c08f660f0bdec099e97ef99f470bb8c607ce9667a165e9caa474710f62ffd")
	signatureTest, _ = hex.DecodeString("8dee56fae60dcad960f7176d0813d5415b930cf6e20c299ec2c2dfc5f2ad4903916fd462ba1abf5c32a5bfd94dcc8eba062d011a548d99df7fa1e3bbbc9a0455663d60f6ccc736c1d5b6de727dbe4427e21fb660925518be386265913f447c94")
	dpkTest, _ = hex.DecodeString("a02fcd15edd52c8e134027491a43b597505b466d1679e88f70f927e57c45a93ae0765ff02fc2d015e3a02fd8748e2103")
	timestampTest = time.Now()

	rand, _ := collectiveBeacon.GetRandomness(signatureTest)
	randomnessTest = &state.Randomness{
		Round:      1,
		Randomness: rand,
		Timestamp:  timestampTest,
	}

	kp := ed25119.GenerateKeyPair()
	issuerPK = kp.PublicKey

	committeeTest = &state.Committee{
		InstanceID:    1,
		Threshold:     3,
		Identities:    []ed25119.PublicKey{issuerPK},
		DistributedPK: dpkTest,
	}
}

func dummyPayload() *cbPayload.Payload {
	header := header.New(header.CollectiveBeaconType(), 1)
	return cbPayload.New(header.Instance(),
		1,
		prevSignatureTest,
		signatureTest,
		dpkTest)
}

func TestDispatcher(t *testing.T) {
	marshalUtil := marshalutil.New(dummyPayload().Bytes())
	parsedPayload, err := payload.Parse(marshalUtil)
	require.NoError(t, err)

	drng := New(state.SetCommittee(committeeTest))
	err = drng.Dispatch(issuerPK, timestampTest, parsedPayload)
	require.NoError(t, err)
	require.Equal(t, *randomnessTest, drng.State.Randomness())
}
