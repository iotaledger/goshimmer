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
	keyPair := ed25519.GenerateKeyPair()
	obj := NewAliasAddress(keyPair.PublicKey.Bytes())

	assert.Equal(t, obj.BytesOld(), obj.Bytes())
}

func TestSerixBLSAddress(t *testing.T) {
	privateKey := bls.PrivateKeyFromRandomness()
	obj := NewBLSAddress(privateKey.PublicKey().Bytes())

	assert.Equal(t, obj.BytesOld(), obj.Bytes())
}

func TestSerixED25519Address(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	obj := NewED25519Address(keyPair.PublicKey)

	assert.Equal(t, obj.BytesOld(), obj.Bytes())
}

func TestSerixAliasOutput(t *testing.T) {
	// uses encode
	data := []byte("dummy")
	obj, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randAliasAddress(), data)
	assert.NoError(t, err)

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	// too complex for serix
	assert.Equal(t, obj.ObjectStorageValue(), obj.ObjectStorageValue())

	objRestored, err := new(AliasOutput).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*AliasOutput).Bytes())
}

func TestSerixExtendedLockedOutput(t *testing.T) {
	// uses encode
	obj := NewExtendedLockedOutput(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	// too complex for serix
	assert.Equal(t, obj.ObjectStorageValue(), obj.ObjectStorageValue())

	objRestored, err := new(ExtendedLockedOutput).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*ExtendedLockedOutput).Bytes())
}

func TestSerixSigLockedColoredOutput(t *testing.T) {
	// OrderedMap uses encode
	obj := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}), randEd25119Address())
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(SigLockedColoredOutput).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*SigLockedColoredOutput).Bytes())
}

func TestSerixSigLockedSingleOutput(t *testing.T) {
	obj := NewSigLockedSingleOutput(10, randEd25119Address())
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(SigLockedSingleOutput).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*SigLockedSingleOutput).Bytes())
}

func TestSerixBranch(t *testing.T) {
	obj := NewBranch(BranchID{1}, NewBranchIDs(BranchID{2}, BranchID{3}), NewConflictIDs(ConflictID{5}, ConflictID{4}))

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(Branch).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*Branch).Bytes())
}

func TestSerixChildBranch(t *testing.T) {
	obj := NewChildBranch(BranchID{1}, BranchID{2})

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValue(), obj.ObjectStorageValue())

	objRestored, err := new(ChildBranch).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*ChildBranch).Bytes())
}

func TestSerixConflict(t *testing.T) {
	obj := NewConflict(ConflictID{1})

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(Conflict).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*Conflict).Bytes())
}

func TestSerixConflictMember(t *testing.T) {
	obj := NewConflictMember(ConflictID{1}, BranchID{2})

	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValue(), obj.ObjectStorageValue())

	objRestored, err := new(ConflictMember).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*ConflictMember).Bytes())
}

func TestSerixBLSSignature(t *testing.T) {
	keyPair := bls.PrivateKeyFromRandomness()
	// uses encode because Signature struct is in hive.go
	signature, err := keyPair.Sign(keyPair.PublicKey().Bytes())
	assert.NoError(t, err)

	obj := NewBLSSignature(bls.NewSignatureWithPublicKey(keyPair.PublicKey(), signature.Signature))

	assert.Equal(t, obj.BytesOld(), obj.Bytes())
	objRestored, _, err := BLSSignatureFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}

func TestSerixED25519Signature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	obj := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := ED25519SignatureFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())

}

func TestSerixAliasUnlockBlock(t *testing.T) {
	obj := NewAliasUnlockBlock(1)

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := AliasUnlockBlockFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}

func TestSerixSignatureUnlockBlock(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	signature := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))
	obj := NewSignatureUnlockBlock(signature)

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := SignatureUnlockBlockFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}

func TestSerixReferenceUnlockBlock(t *testing.T) {
	obj := NewReferenceUnlockBlock(1)

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := ReferenceUnlockBlockFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}

func TestSerixUTXOInput(t *testing.T) {
	obj := NewUTXOInput(randOutputID())

	assert.Equal(t, obj.BytesOld(), obj.Bytes())
}
func TestSerixTransactionMetadata(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), tx, serix.WithValidation())
	assert.NoError(t, err)
	fmt.Println(objBytes)
	obj := NewTransactionMetadata(tx.ID())
	obj.SetSolid(true)
	obj.SetGradeOfFinality(gof.High)
	obj.SetBranchIDs(NewBranchIDs(BranchIDFromRandomness()))
	obj.SetLazyBooked(false)

	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())

	objRestored, err := new(TransactionMetadata).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*TransactionMetadata).Bytes())
}

func TestSerixTransactionEssence(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)

	assert.Equal(t, tx.Essence().BytesOld(), tx.Essence().Bytes())

	objRestored, consumedBytes, err := TransactionEssenceFromBytes(tx.Essence().Bytes())
	assert.NoError(t, err)
	assert.Equal(t, len(tx.Essence().Bytes()), consumedBytes)
	assert.Equal(t, tx.Essence().Bytes(), objRestored.Bytes())
}

func TestSerixTransactionPayload(t *testing.T) {
	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)

	// skip payload length which is not written when serializing transaction directly
	assert.Equal(t, tx.BytesOld(), tx.Bytes())
	assert.Equal(t, tx.ObjectStorageValueOld(), tx.ObjectStorageValue())
	assert.Equal(t, tx.ObjectStorageKeyOld(), tx.ObjectStorageKey())

	objRestored, err := new(Transaction).FromObjectStorage(tx.ObjectStorageKey(), tx.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, tx.Bytes(), objRestored.(*Transaction).Bytes())
}

func TestSerixBranchIDs(t *testing.T) {
	obj := NewBranchIDs(BranchIDFromRandomness())
	assert.Equal(t, obj.BytesOld(), obj.Bytes())
}

func TestSerixOutputMetadata(t *testing.T) {
	obj := NewOutputMetadata(randOutputID())
	obj.AddBranchID(BranchIDFromRandomness())
	obj.SetGradeOfFinality(gof.High)
	obj.SetSolid(true)
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(OutputMetadata).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*OutputMetadata).Bytes())
}

func TestSerixConsumer(t *testing.T) {
	obj := NewConsumer(randOutputID(), GenesisTransactionID, types.Maybe)
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	objRestored, err := new(Consumer).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*Consumer).Bytes())
}

func TestSerixAddressOutputMapping(t *testing.T) {
	obj := NewAddressOutputMapping(randEd25119Address(), randOutputID())
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())

	objRestored, err := new(AddressOutputMapping).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.(*AddressOutputMapping).Bytes())
}
