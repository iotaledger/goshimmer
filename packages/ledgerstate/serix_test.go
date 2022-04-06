package ledgerstate

import (
	"context"
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
)

func TestSerixAliasAddress(t *testing.T) {
	// uses encode
	keyPair := ed25519.GenerateKeyPair()
	obj := NewAliasAddress(keyPair.PublicKey.Bytes())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixBLSAddress(t *testing.T) {
	privateKey := bls.PrivateKeyFromRandomness()
	obj := NewBLSAddress(privateKey.PublicKey().Bytes())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixED25519Address(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	obj := NewED25519Address(keyPair.PublicKey)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixAliasOutput(t *testing.T) {
	// uses encode
	data := []byte("dummy")
	obj, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randAliasAddress(), data)
	assert.NoError(t, err)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixExtendedLockedOutput(t *testing.T) {
	// uses encode
	obj := NewExtendedLockedOutput(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSigLockedColoredOutput(t *testing.T) {
	// OrderedMap uses encode
	obj := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}), randEd25119Address())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSigLockedSingleOutput(t *testing.T) {
	sigLockedSingleOutput := NewSigLockedSingleOutput(10, randEd25119Address())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), sigLockedSingleOutput)
	assert.NoError(t, err)
	assert.Equal(t, sigLockedSingleOutput.Bytes(), serixBytes)
}

func TestSerixBranch(t *testing.T) {
	branch := NewBranch(BranchID{1}, NewBranchIDs(BranchID{2}, BranchID{3}), NewConflictIDs(ConflictID{5}, ConflictID{4}))

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), branch)
	assert.NoError(t, err)
	assert.Equal(t, branch.Bytes(), serixBytes)
}

func TestSerixChildBranch(t *testing.T) {
	childBranch := NewChildBranch(BranchID{1}, BranchID{2})

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), childBranch)
	assert.NoError(t, err)
	assert.Equal(t, childBranch.Bytes(), serixBytes)
}

func TestSerixConflict(t *testing.T) {
	conflict := NewConflict(ConflictID{1})

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), conflict)
	assert.NoError(t, err)
	assert.Equal(t, conflict.Bytes(), serixBytes)
}

func TestSerixConflictMember(t *testing.T) {
	conflictMember := NewConflictMember(ConflictID{1}, BranchID{2})

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), conflictMember)
	assert.NoError(t, err)
	assert.Equal(t, conflictMember.Bytes(), serixBytes)
}

func TestSerixBLSSignature(t *testing.T) {
	keyPair := bls.PrivateKeyFromRandomness()
	// uses encode because Signature struct is in hive.go
	signature, err := keyPair.Sign(keyPair.PublicKey().Bytes())
	assert.NoError(t, err)

	obj := NewBLSSignature(bls.NewSignatureWithPublicKey(keyPair.PublicKey(), signature.Signature))

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixED25519Signature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	obj := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixAliasUnlockBlock(t *testing.T) {
	obj := NewAliasUnlockBlock(1)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSignatureUnlockBlock(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	signature := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))
	obj := NewSignatureUnlockBlock(signature)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixReferenceUnlockBlock(t *testing.T) {
	obj := NewReferenceUnlockBlock(1)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixUTXOInput(t *testing.T) {
	obj := NewUTXOInput(randOutputID())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}
func TestSerixTransactionMetadata(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)

	obj := NewTransactionMetadata(tx.ID())
	obj.SetSolid(true)
	obj.SetGradeOfFinality(gof.High)
	obj.SetBranchIDs(NewBranchIDs(BranchIDFromRandomness()))
	obj.SetLazyBooked(false)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)

	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj.ID())
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)
}

func TestSerixTransactionPayload(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)
	serializedBytes, err := serix.DefaultAPI.Encode(context.Background(), tx)
	assert.NoError(t, err)

	// skip payload length which is not written when serializing transaction directly
	assert.Equal(t, tx.Bytes()[4:], serializedBytes)
}

func TestSerixBranchIDs(t *testing.T) {
	obj := NewBranchIDs(BranchIDFromRandomness())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	fmt.Println("Bytes", len(obj.Bytes()), obj.Bytes())
	fmt.Println("Serix", len(serixBytes), serixBytes)
	assert.Equal(t, obj.Bytes(), serixBytes)

}

func TestSerixOutputMetadata(t *testing.T) {
	obj := NewOutputMetadata(randOutputID())
	obj.AddBranchID(BranchIDFromRandomness())
	obj.SetGradeOfFinality(gof.High)
	obj.SetSolid(true)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID which is serialized by the Bytes method
	assert.Equal(t, obj.Bytes()[34:], serixBytes)
}

func TestSerixConsumer(t *testing.T) {
	obj := NewConsumer(randOutputID(), GenesisTransactionID, types.Maybe)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID and TransactionID which are serialized by the Bytes method, but are used only as a object storage key.
	assert.Equal(t, obj.Bytes()[66:], serixBytes)
}

func TestSerixAddressOutputMapping(t *testing.T) {
	obj := NewAddressOutputMapping(randEd25119Address(), randOutputID())

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID and TransactionID which are serialized by the Bytes method, but are used only as a object storage key.
	assert.Equal(t, obj.Bytes(), serixBytes)
}
