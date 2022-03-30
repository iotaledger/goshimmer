package ledgerstate

import (
	"context"
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/crypto/bls"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func TestSerixAliasAddress(t *testing.T) {
	// uses encode
	keyPair := ed25519.GenerateKeyPair()
	obj := NewAliasAddress(keyPair.PublicKey.Bytes())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(AliasAddress), serix.TypeSettings{}.WithObjectCode(new(AliasAddress).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(AliasAddress))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixBLSAddress(t *testing.T) {
	privateKey := bls.PrivateKeyFromRandomness()
	obj := NewBLSAddress(privateKey.PublicKey().Bytes())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(BLSAddress), serix.TypeSettings{}.WithObjectCode(new(BLSAddress).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(BLSAddress))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixED25519Address(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	obj := NewED25519Address(keyPair.PublicKey)

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixAliasOutput(t *testing.T) {
	// uses encode
	data := []byte("dummy")
	obj, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randAliasAddress(), data)
	assert.NoError(t, err)

	s := serix.NewAPI()
	err = s.RegisterTypeSettings(new(AliasAddress), serix.TypeSettings{}.WithObjectCode(new(AliasAddress).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(AliasAddress))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixExtendedLockedOutput(t *testing.T) {
	// uses encode
	obj := NewExtendedLockedOutput(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}
func TestSerixSigLockedColoredOutput(t *testing.T) {
	// OrderedMap uses encode
	obj := NewSigLockedColoredOutput(NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}), randEd25119Address())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(SigLockedColoredOutput), serix.TypeSettings{}.WithObjectCode(new(SigLockedColoredOutput).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Output)(nil), new(SigLockedColoredOutput))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSigLockedSingleOutput(t *testing.T) {
	sigLockedSingleOutput := NewSigLockedSingleOutput(10, randEd25119Address())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(SigLockedSingleOutput), serix.TypeSettings{}.WithObjectCode(new(SigLockedSingleOutput).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Output)(nil), new(SigLockedSingleOutput))
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
	err = s.RegisterTypeSettings(new(BLSSignature), serix.TypeSettings{}.WithObjectCode(new(BLSSignature).Type()))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(ED25519Signature), serix.TypeSettings{}.WithObjectCode(new(ED25519Signature).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixED25519Signature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	obj := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(BLSSignature), serix.TypeSettings{}.WithObjectCode(new(BLSSignature).Type()))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(ED25519Signature), serix.TypeSettings{}.WithObjectCode(new(ED25519Signature).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixAliasUnlockBlock(t *testing.T) {
	obj := NewAliasUnlockBlock(1)

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(AliasUnlockBlock), serix.TypeSettings{}.WithObjectCode(new(AliasUnlockBlock).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*UnlockBlock)(nil), new(AliasUnlockBlock))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSignatureUnlockBlock(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	signature := NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(keyPair.PublicKey.Bytes()))
	obj := NewSignatureUnlockBlock(signature)

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(BLSSignature), serix.TypeSettings{}.WithObjectCode(new(BLSSignature).Type()))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(ED25519Signature), serix.TypeSettings{}.WithObjectCode(new(ED25519Signature).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(SignatureUnlockBlock), serix.TypeSettings{}.WithObjectCode(new(SignatureUnlockBlock).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*UnlockBlock)(nil), new(SignatureUnlockBlock))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixReferenceUnlockBlock(t *testing.T) {
	obj := NewReferenceUnlockBlock(1)

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(ReferenceUnlockBlock), serix.TypeSettings{}.WithObjectCode(new(ReferenceUnlockBlock).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*UnlockBlock)(nil), new(ReferenceUnlockBlock))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixUTXOInput(t *testing.T) {
	obj := NewUTXOInput(randOutputID())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(UTXOInput), serix.TypeSettings{}.WithObjectCode(new(UTXOInput).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Input)(nil), new(UTXOInput))
	assert.NoError(t, err)
	serixBytes, err := s.Encode(context.Background(), obj)
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
	obj.SetBranchIDs(NewBranchIDs(BranchIDFromRandomness(), BranchIDFromRandomness()))
	obj.SetLazyBooked(false)
	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)

	serixBytesKey, err := s.Encode(context.Background(), obj.ID())
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageKey(), serixBytesKey)
}
func TestSerixTransactionPayload(t *testing.T) {
	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(AliasUnlockBlock), serix.TypeSettings{}.WithObjectCode(new(AliasUnlockBlock).Type()))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(ReferenceUnlockBlock), serix.TypeSettings{}.WithObjectCode(new(ReferenceUnlockBlock).Type()))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(SignatureUnlockBlock), serix.TypeSettings{}.WithObjectCode(new(SignatureUnlockBlock).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*UnlockBlock)(nil), new(AliasUnlockBlock), new(ReferenceUnlockBlock), new(SignatureUnlockBlock))
	assert.NoError(t, err)

	err = s.RegisterTypeSettings(new(BLSSignature), serix.TypeSettings{}.WithObjectCode(new(BLSSignature).Type()))
	assert.NoError(t, err)
	err = s.RegisterTypeSettings(new(ED25519Signature), serix.TypeSettings{}.WithObjectCode(new(ED25519Signature).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Signature)(nil), new(BLSSignature), new(ED25519Signature))
	assert.NoError(t, err)

	err = s.RegisterTypeSettings(new(UTXOInput), serix.TypeSettings{}.WithObjectCode(new(UTXOInput).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Input)(nil), new(UTXOInput))
	assert.NoError(t, err)

	err = s.RegisterTypeSettings(new(SigLockedSingleOutput), serix.TypeSettings{}.WithObjectCode(new(SigLockedSingleOutput).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Output)(nil), new(SigLockedSingleOutput))
	assert.NoError(t, err)

	err = s.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)

	err = s.RegisterTypeSettings(new(Transaction), serix.TypeSettings{}.WithObjectCode(new(Transaction).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*payload.Payload)(nil), new(Transaction))
	assert.NoError(t, err)

	ledgerstate := setupDependencies(t)
	defer ledgerstate.Shutdown()
	wallets := createWallets(2)
	input := generateOutput(ledgerstate, wallets[0].address, 0)
	tx, _ := singleInputTransaction(ledgerstate, wallets[0], wallets[1], input)
	serializedBytes, err := s.Encode(context.Background(), tx)
	assert.NoError(t, err)

	// skip payload length which is not written when serializing transaction directly
	assert.Equal(t, tx.Bytes()[4:], serializedBytes)
}
func TestSerixBranchIDs(t *testing.T) {
	obj := NewBranchIDs(BranchIDFromRandomness())

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj, serix.WithTypeSettings(new(serix.TypeSettings).WithLengthPrefixType(serializer.UInt32ByteSize)))
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

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID which is serialized by the Bytes method
	assert.Equal(t, obj.Bytes()[34:], serixBytes)
}

func TestSerixConsumer(t *testing.T) {
	obj := NewConsumer(randOutputID(), GenesisTransactionID, types.Maybe)

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID and TransactionID which are serialized by the Bytes method, but are used only as a object storage key.
	assert.Equal(t, obj.Bytes()[66:], serixBytes)
}

func TestSerixAddressOutputMapping(t *testing.T) {
	obj := NewAddressOutputMapping(randEd25119Address(), randOutputID())

	s := serix.NewAPI()
	err := s.RegisterTypeSettings(new(ED25519Address), serix.TypeSettings{}.WithObjectCode(new(ED25519Address).Type()))
	assert.NoError(t, err)
	err = s.RegisterInterfaceObjects((*Address)(nil), new(ED25519Address))
	assert.NoError(t, err)

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID and TransactionID which are serialized by the Bytes method, but are used only as a object storage key.
	assert.Equal(t, obj.Bytes(), serixBytes)
}
