package value

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

func TestNewTransactionFromJSON(t *testing.T) {
	mySeed := walletseed.NewSeed()
	myOutputID := "2ZU8TNkVVGKmbFqifhejufMqpaKcSMAUvGadW4igVXB87rP"
	tokenColorStr := "E113QN5qZbTEvqwtGxeEmTZdBCsCbb6waYK9WDCse3Aw"

	out, err := ledgerstate.OutputIDFromBase58(myOutputID)
	require.NoError(t, err)
	tokenColor, err := ledgerstate.ColorFromBase58EncodedString(tokenColorStr)
	require.NoError(t, err)

	// create a new receiver wallet for the given conflict
	receiverSeeds := walletseed.NewSeed()
	destAddr := receiverSeeds.Address(0)

	output1 := ledgerstate.NewSigLockedSingleOutput(uint64(100), destAddr.Address())
	output2 := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		tokenColor:            uint64(100),
		ledgerstate.ColorMint: uint64(100),
	}), destAddr.Address())

	// nodeID to pledge mana
	pledge, _ := identity.RandomID()
	pledgeID := make([]byte, hex.EncodedLen(len(pledge.Bytes())))
	_ = hex.Encode(pledgeID, pledge.Bytes())

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), pledge, pledge, ledgerstate.NewInputs(ledgerstate.NewUTXOInput(out)), ledgerstate.NewOutputs(output1, output2))
	// create data payload
	dataPayload := payload.NewGenericDataPayload([]byte("some data"))
	txEssence.SetPayload(dataPayload)

	// sign transaction
	kp := *mySeed.KeyPair(0)
	sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))

	// create unlockBlock
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	// create tx
	tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})

	// output JSON object
	outputsObj := []value.Output{
		{
			Type:    output1.Type(),
			Address: output1.Address().Base58(),
			Balances: []value.Balance{
				{
					Value: 100,
					Color: "IOTA",
				},
			},
		},
		{
			Type:    output2.Type(),
			Address: output2.Address().Base58(),
			Balances: []value.Balance{
				{
					Value: 100,
					Color: "MINT",
				},
				{
					Value: 100,
					Color: tokenColorStr,
				},
			},
		},
	}

	// signature JSON object
	sigObj := value.UnlockBlock{
		Type:          ledgerstate.SignatureUnlockBlockType,
		SignatureType: ledgerstate.ED25519SignatureType,
		PublicKey:     kp.PublicKey.String(),
		Signature:     sig.Base58(),
	}

	req := value.SendTransactionByJSONRequest{
		Inputs:        []string{myOutputID},
		Outputs:       outputsObj,
		Signatures:    []value.UnlockBlock{sigObj},
		AManaPledgeID: string(pledgeID),
		CManaPledgeID: string(pledgeID),
		Payload:       dataPayload.Bytes(),
	}

	txFromJSON, err := NewTransactionFromJSON(req)
	require.NoError(t, err)

	assert.Equal(t, tx.ReferencedTransactionIDs(), txFromJSON.ReferencedTransactionIDs())
	assert.Equal(t, tx.UnlockBlocks()[0].Bytes(), txFromJSON.UnlockBlocks()[0].Bytes())
	assert.Equal(t, tx.Essence().AccessPledgeID(), txFromJSON.Essence().AccessPledgeID())
	assert.Equal(t, tx.Essence().ConsensusPledgeID(), txFromJSON.Essence().ConsensusPledgeID())
	assert.Equal(t, tx.Essence().Payload(), txFromJSON.Essence().Payload())
	assert.Equal(t, tx.Essence().Inputs(), txFromJSON.Essence().Inputs())
	assert.Equal(t, tx.Essence().Outputs().Bytes(), txFromJSON.Essence().Outputs().Bytes())
}
