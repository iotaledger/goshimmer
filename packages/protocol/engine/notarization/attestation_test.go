package notarization

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
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

func TestAttestations(t *testing.T) {
	timeProvider := slot.NewTimeProvider(time.Now().Unix(), 10)
	tf := newAttestationsTestFramework(t, timeProvider)

	// create issuers
	tf.CreateIssuer("Batman", 10)
	tf.CreateIssuer("Robin", 10)
	tf.CreateIssuer("Joker", 10)
	tf.CreateIssuer("Superman", 10)

	// create blocks
	tf.CreateBlock("1.1", 1)
	tf.CreateBlock("2.1", 2, models.WithIssuer(tf.Issuer("Batman")))
	tf.CreateBlock("2.2", 2, models.WithIssuer(tf.Issuer("Robin")))
	tf.CreateBlock("2.3", 2, models.WithIssuer(tf.Issuer("Joker")))
	tf.CreateBlock("2.4", 2, models.WithIssuer(tf.Issuer("Superman")))
	tf.CreateBlock("2.5", 2, models.WithIssuer(tf.Issuer("Superman")))

	// slot 1
	tf.AddBlock(t, "1.1")
	tf.ValidateCommit(t, 1)

	// slot 2
	tf.AddBlock(t, "2.1")
	tf.AddBlock(t, "2.2")
	tf.AddBlock(t, "2.3")
	tf.RemoveBlock(t, "2.1")
	tf.AddBlock(t, "2.4")
	tf.AddBlock(t, "2.5")
	tf.ValidateCommit(t, 2)
	w, err := tf.attestations.Weight(2)
	require.NoError(t, err)
	require.Equal(t, int64(30), w)

	// try to get uncommitted slot 3
	_, err = tf.attestations.Get(3)
	require.Error(t, err)
}

type attestationsTestFramework struct {
	*TestFramework
	attestations         *Attestations
	expectedAttestations map[slot.Index]map[identity.ID]*Attestation
}

func newAttestationsTestFramework(t *testing.T, slotTimeProvider *slot.TimeProvider) *attestationsTestFramework {
	tf := NewTestFramework(t, slotTimeProvider)

	baseDir := t.TempDir()
	testStorage := storage.New(baseDir, 1)
	attestations := NewAttestations(testStorage.Permanent.Attestations, testStorage.Prunable.Attestations, tf.weights, slotTimeProvider)

	return &attestationsTestFramework{
		TestFramework:        tf,
		attestations:         attestations,
		expectedAttestations: make(map[slot.Index]map[identity.ID]*Attestation),
	}
}

func (t *attestationsTestFramework) AddBlock(test *testing.T, alias string) {
	block := t.Block(alias)
	if block == nil {
		panic("block does not exist")
	}

	a := NewAttestation(block, t.slotTimeProvider)
	added, err := t.attestations.Add(a)
	require.NoError(test, err)
	require.True(test, added)

	if _, ok := t.expectedAttestations[block.ID().Index()]; !ok {
		t.expectedAttestations[block.ID().Index()] = make(map[identity.ID]*Attestation)
	}
	t.expectedAttestations[block.ID().Index()][block.IssuerID()] = a
}

func (t *attestationsTestFramework) RemoveBlock(test *testing.T, alias string) {
	block := t.Block(alias)
	if block == nil {
		panic("block does not exist")
	}

	a := NewAttestation(block, t.slotTimeProvider)
	deleted, err := t.attestations.Delete(a)
	require.NoError(test, err)
	require.True(test, deleted)

	delete(t.expectedAttestations[block.ID().Index()], block.IssuerID())
}

func (t *attestationsTestFramework) ValidateCommit(test *testing.T, index slot.Index) {
	ats, _, err := t.attestations.Commit(index)
	require.NoError(test, err)

	ats.Stream(func(key identity.ID, value *Attestation) bool {
		_, ok := t.expectedAttestations[index][value.IssuerID()]
		require.True(test, ok)
		return true
	})
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
