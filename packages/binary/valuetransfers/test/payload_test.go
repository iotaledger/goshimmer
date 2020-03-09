package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/coloredbalance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/coloredbalance/color"
	valuepayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer"
	transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/inputs"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/outputs"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/signatures"
	transferoutputid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/transferoutput/id"
)

func TestPayload(t *testing.T) {
	addressKeyPair1 := ed25119.GenerateKeyPair()
	addressKeyPair2 := ed25119.GenerateKeyPair()

	originalPayload := valuepayload.New(
		payloadid.Empty,
		payloadid.Empty,
		transfer.New(
			inputs.New(
				transferoutputid.New(address.FromED25519PubKey(addressKeyPair1.PublicKey), transferid.New([]byte("transfer1"))),
				transferoutputid.New(address.FromED25519PubKey(addressKeyPair2.PublicKey), transferid.New([]byte("transfer2"))),
			),

			outputs.New(map[address.Address][]*coloredbalance.ColoredBalance{
				address.New([]byte("output_address")): {
					coloredbalance.New(color.IOTA, 1337),
				},
			}),
		).Sign(
			signatures.ED25519(addressKeyPair1),
		),
	)

	assert.Equal(t, false, originalPayload.GetTransfer().SignaturesValid())

	originalPayload.GetTransfer().Sign(
		signatures.ED25519(addressKeyPair2),
	)

	assert.Equal(t, true, originalPayload.GetTransfer().SignaturesValid())

	clonedPayload1, err, _ := valuepayload.FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetId(), clonedPayload1.GetId())
	assert.Equal(t, true, clonedPayload1.GetTransfer().SignaturesValid())

	clonedPayload2, err, _ := valuepayload.FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetId(), clonedPayload2.GetId())
	assert.Equal(t, true, clonedPayload2.GetTransfer().SignaturesValid())
}
