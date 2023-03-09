package notarization

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

func TestAttestation_Serialization(t *testing.T) {
	timeProvider := slot.NewTimeProvider(time.Now().Unix(), 10)
	blk := createBlock(timeProvider)

	a := NewAttestation(blk, timeProvider)
	valid, err := a.VerifySignature()
	require.NoError(t, err)
	require.True(t, valid)

	serializedBytes, err := a.Bytes()
	require.NoError(t, err)

	var aDeserialized Attestation
	decodedBytes, err := aDeserialized.FromBytes(serializedBytes)
	require.NoError(t, err)
	require.Equal(t, len(serializedBytes), decodedBytes)

	valid, err = aDeserialized.VerifySignature()
	require.NoError(t, err)
	require.True(t, valid)

	require.Equal(t, a.BlockContentHash, aDeserialized.BlockContentHash)
	require.Equal(t, a.CommitmentID, aDeserialized.CommitmentID)
	require.Equal(t, a.IssuerPublicKey, a.IssuerPublicKey)
	require.Equal(t, a.IssuingTime, a.IssuingTime)
	require.Equal(t, a.Signature, aDeserialized.Signature)
	require.Equal(t, a.IssuerID(), aDeserialized.IssuerID())

}

func createBlock(slotProvider *slot.TimeProvider) *models.Block {
	parents := models.NewBlockIDs()
	var parent models.BlockID
	_ = parent.FromRandomness()
	parents.Add(parent)

	keyPair := ed25519.GenerateKeyPair()
	blk := models.NewBlock(models.WithStrongParents(parents))
	blk.Sign(&keyPair)

	blk.DetermineID(slotProvider)

	return blk
}
