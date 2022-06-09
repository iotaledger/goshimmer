package faucet

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func ExampleRequest() {
	keyPair := ed25519.GenerateKeyPair()
	address := devnetvm.NewED25519Address(keyPair.PublicKey)
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)
	emptyID := identity.ID{}

	// 1. create faucet payload
	faucetRequest := NewRequest(address, emptyID, emptyID, 0)

	// 2. build actual message
	tx := tangle.NewMessage(map[tangle.ParentsType]tangle.MessageIDs{
		tangle.StrongParentType: {
			tangle.EmptyMessageID: types.Void,
		},
	},
		time.Now(),
		local.PublicKey(),
		0,
		faucetRequest,
		0,
		ed25519.EmptySignature,
		0,
		nil,
	)
	fmt.Println(tx.String())
}

func TestRequest(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	address := devnetvm.NewED25519Address(keyPair.PublicKey)
	access, _ := identity.RandomID()
	consensus, _ := identity.RandomID()

	originalRequest := NewRequest(address, access, consensus, 0)

	clonedRequest, _, err := FromBytes(lo.PanicOnErr(originalRequest.Bytes()))
	if err != nil {
		panic(err)
	}
	assert.Equal(t, originalRequest.Address(), clonedRequest.Address())
	assert.Equal(t, originalRequest.AccessManaPledgeID(), clonedRequest.AccessManaPledgeID())
	assert.Equal(t, originalRequest.ConsensusManaPledgeID(), clonedRequest.ConsensusManaPledgeID())

	clonedRequest2, _, err := FromBytes(lo.PanicOnErr(clonedRequest.Bytes()))
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalRequest.Address(), clonedRequest2.Address())
}

func TestIsFaucetReq(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	address := devnetvm.NewED25519Address(keyPair.PublicKey)
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)
	emptyID := identity.ID{}

	faucetRequest := NewRequest(address, emptyID, emptyID, 0)

	faucetMsg := tangle.NewMessage(
		map[tangle.ParentsType]tangle.MessageIDs{
			tangle.StrongParentType: {
				tangle.EmptyMessageID: types.Void,
			},
		},
		time.Now(),
		local.PublicKey(),
		0,
		faucetRequest,
		0,
		ed25519.EmptySignature,
		0,
		nil,
	)

	dataMsg := tangle.NewMessage(
		map[tangle.ParentsType]tangle.MessageIDs{
			tangle.StrongParentType: {
				tangle.EmptyMessageID: types.Void,
			},
		},
		time.Now(),
		local.PublicKey(),
		0,
		payload.NewGenericDataPayload([]byte("data")),
		0,
		ed25519.EmptySignature,
		0,
		nil,
	)

	assert.Equal(t, true, IsFaucetReq(faucetMsg))
	assert.Equal(t, false, IsFaucetReq(dataMsg))
}
