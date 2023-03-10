package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
)

func TestRequest(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	address := devnetvm.NewED25519Address(keyPair.PublicKey)
	access, _ := identity.RandomIDInsecure()
	consensus, _ := identity.RandomIDInsecure()

	originalRequest := NewRequest(address, access, consensus, 0)

	clonedRequest, _, err := FromBytes(lo.PanicOnErr(originalRequest.Bytes()))
	if err != nil {
		panic(err)
	}
	require.Equal(t, originalRequest.Address(), clonedRequest.Address())
	require.Equal(t, originalRequest.AccessManaPledgeID(), clonedRequest.AccessManaPledgeID())
	require.Equal(t, originalRequest.ConsensusManaPledgeID(), clonedRequest.ConsensusManaPledgeID())

	clonedRequest2, _, err := FromBytes(lo.PanicOnErr(clonedRequest.Bytes()))
	if err != nil {
		panic(err)
	}

	require.Equal(t, originalRequest.Address(), clonedRequest2.Address())
}

func TestIsFaucetReq(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	address := devnetvm.NewED25519Address(keyPair.PublicKey)
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)
	emptyID := identity.ID{}

	faucetRequest := NewRequest(address, emptyID, emptyID, 0)

	faucetBlk := models.NewBlock(
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
		models.WithIssuingTime(time.Now()),
		models.WithIssuer(local.PublicKey()),
		models.WithSequenceNumber(0),
		models.WithPayload(faucetRequest),
		models.WithNonce(0),
		models.WithSignature(ed25519.EmptySignature),
		models.WithLatestConfirmedSlot(0),
		models.WithCommitment(commitment.New(0, commitment.ID{}, types.Identifier{}, 0)),
	)

	dataBlk := models.NewBlock(
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
		models.WithIssuingTime(time.Now()),
		models.WithIssuer(local.PublicKey()),
		models.WithSequenceNumber(0),
		models.WithPayload(payload.NewGenericDataPayload([]byte("data"))),
		models.WithNonce(0),
		models.WithSignature(ed25519.EmptySignature),
		models.WithLatestConfirmedSlot(0),
		models.WithCommitment(commitment.New(0, commitment.ID{}, types.Identifier{}, 0)),
	)

	require.Equal(t, true, IsFaucetReq(faucetBlk))
	require.Equal(t, false, IsFaucetReq(dataBlk))
}
