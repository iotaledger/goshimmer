package ledgerstate

import (
	"context"
	"testing"

	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serix"
	"github.com/stretchr/testify/assert"
)

func TestSerixAliasAddress(t *testing.T) {
	// uses encode
	keyPair := ed25519.GenerateKeyPair()
	obj := NewAliasAddress(keyPair.PublicKey.Bytes())

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixBLSAddress(t *testing.T) {
	privateKey := bls.PrivateKeyFromRandomness()
	obj := NewBLSAddress(privateKey.PublicKey().Bytes())

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixED25519Address(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	obj := NewED25519Address(keyPair.PublicKey)

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixAliasOutput(t *testing.T) {
	// uses encode
	data := []byte("dummy")
	obj, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randAliasAddress(), data)

	s := serix.NewAPI()
	err = s.RegisterObjects((*Address)(nil), new(AliasAddress))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixExtendedLockedOutput(t *testing.T) {
	obj := NewExtendedLockedOutput(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())

	s := serix.NewAPI()
	err := s.RegisterObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}
func TestSerixSigLockedColoredOutput(t *testing.T) {
	// OrderedMap uses encode
	obj := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}), randEd25119Address())

	s := serix.NewAPI()
	err := s.RegisterObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSigLockedSingleOutput(t *testing.T) {
	sigLockedSingleOutput := NewSigLockedSingleOutput(10, randEd25119Address())

	s := serix.NewAPI()

	err := s.RegisterObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), sigLockedSingleOutput)
	assert.NoError(t, err)
	assert.Equal(t, sigLockedSingleOutput.Bytes(), serixBytes)
}

func TestSerixBranch(t *testing.T) {
	branch := NewBranch(BranchID{1}, NewBranchIDs(BranchID{2}, BranchID{3}), NewConflictIDs(ConflictID{5}, ConflictID{4}))

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), branch)
	assert.NoError(t, err)
	assert.Equal(t, branch.Bytes(), serixBytes)
}

func TestSerixChildBranch(t *testing.T) {
	childBranch := NewChildBranch(BranchID{1}, BranchID{2})

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), childBranch)
	assert.NoError(t, err)
	assert.Equal(t, childBranch.Bytes(), serixBytes)
}

func TestSerixConflict(t *testing.T) {
	conflict := NewConflict(ConflictID{1})

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), conflict)
	assert.NoError(t, err)
	assert.Equal(t, conflict.Bytes(), serixBytes)
}

func TestSerixConflictMember(t *testing.T) {
	conflictMember := NewConflictMember(ConflictID{1}, BranchID{2})

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), conflictMember)
	assert.NoError(t, err)
	assert.Equal(t, conflictMember.Bytes(), serixBytes)
}

func TestSerixBLSSignature(t *testing.T) {
	keyPair := bls.PrivateKeyFromRandomness()
	// uses encode because Signature struct is in hive.go
	signature, err := keyPair.Sign(keyPair.PublicKey().Bytes())
	assert.NoError(t, err)

	obj := NewBLSSignature(bls.NewSignatureWithPublicKey(keyPair.PublicKey(), signature.Signature))

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixED25519Signature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	obj := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixAliasUnlockBlock(t *testing.T) {
	obj := NewAliasUnlockBlock(1)

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSignatureUnlockBlock(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	signature := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))
	obj := NewSignatureUnlockBlock(signature)

	s := serix.NewAPI()
	err := s.RegisterObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixReferenceUnlockBlock(t *testing.T) {
	obj := NewReferenceUnlockBlock(1)

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixUTXOInput(t *testing.T) {
	obj := NewUTXOInput(randOutputID())

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixTransactionPayload(t *testing.T) {
	s := serix.NewAPI()
	err := s.RegisterObjects((*UnlockBlock)(nil), new(AliasUnlockBlock), new(ReferenceUnlockBlock), new(SignatureUnlockBlock))
	assert.NoError(t, err)

	err = s.RegisterObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	assert.NoError(t, err)

	err = s.RegisterObjects((*Input)(nil), new(UTXOInput))
	assert.NoError(t, err)

	err = s.RegisterObjects((*Output)(nil), new(SigLockedSingleOutput))
	assert.NoError(t, err)

	err = s.RegisterObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)

	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)
	serializedBytes, err := s.Encode(context.Background(), tx)
	assert.NoError(t, err)

	// skip payload length which is not written when serializing transaciton directly
	assert.Equal(t, tx.Bytes()[4:], serializedBytes)
}
