package ledgerstate

import (
	"bytes"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
	"sync"
)

// AliasLockedOutput is an Output that can be unlocked by reference to the unlocked
type AliasLockedOutput struct {
	id      OutputID
	idMutex sync.RWMutex
	balance uint64
	address Address

	objectstorage.StorableObjectFlags
}

// NewAliasLockedOutput is the constructor for a AliasLockedOutput.
func NewAliasLockedOutput(balance uint64, address Address) *AliasLockedOutput {
	return &AliasLockedOutput{
		balance: balance,
		address: address,
	}
}

// AliasLockedOutputFromBytes unmarshals a AliasLockedOutput from a sequence of bytes.
func AliasLockedOutputFromBytes(bytes []byte) (output *AliasLockedOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = AliasLockedOutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AliasLockedOutput from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AliasLockedOutputFromMarshalUtil unmarshals a AliasLockedOutput using a MarshalUtil (for easier unmarshaling).
func AliasLockedOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *AliasLockedOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != AliasLockedOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	output = &AliasLockedOutput{}
	if output.balance, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse balance (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if output.balance < MinOutputBalance {
		err = xerrors.Errorf("balance (%d) is smaller than MinOutputBalance (%d): %w", output.balance, MinOutputBalance, cerrors.ErrParseBytesFailed)
		return
	}
	if output.balance > MaxOutputBalance {
		err = xerrors.Errorf("balance (%d) is bigger than MaxOutputBalance (%d): %w", output.balance, MaxOutputBalance, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (s *AliasLockedOutput) ID() OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *AliasLockedOutput) SetID(outputID OutputID) Output {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.id = outputID

	return s
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (s *AliasLockedOutput) Type() OutputType {
	return AliasLockedOutputType
}

// Balances returns the funds that are associated with the Output.
func (s *AliasLockedOutput) Balances() *ColoredBalances {
	balances := NewColoredBalances(map[Color]uint64{
		ColorIOTA: s.balance,
	})

	return balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (s *AliasLockedOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock) (unlockValid bool, err error) {
	signatureUnlockBlock, correctType := unlockBlock.(*SignatureUnlockBlock)
	if !correctType {
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", cerrors.ErrParseBytesFailed)
		return
	}

	unlockValid = signatureUnlockBlock.AddressSignatureValid(s.address, tx.Essence().Bytes())

	return
}

// Address returns the Address that the Output is associated to.
func (s *AliasLockedOutput) Address() Address {
	return s.address
}

// Input returns an Input that references the Output.
func (s *AliasLockedOutput) Input() Input {
	if s.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID yet cannot be converted to an Input")
	}

	return NewUTXOInput(s.ID())
}

// Clone creates a copy of the Output.
func (s *AliasLockedOutput) Clone() Output {
	return &AliasLockedOutput{
		id:      s.id,
		balance: s.balance,
		address: s.address.Clone(),
	}
}

// Bytes returns a marshaled version of the Output.
func (s *AliasLockedOutput) Bytes() []byte {
	return s.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *AliasLockedOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *AliasLockedOutput) ObjectStorageKey() []byte {
	return s.ID().Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (s *AliasLockedOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(AliasLockedOutputType)).
		WriteUint64(s.balance).
		WriteBytes(s.address.Bytes()).
		Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (s *AliasLockedOutput) Compare(other Output) int {
	return bytes.Compare(s.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (s *AliasLockedOutput) String() string {
	return stringify.Struct("AliasLockedOutput",
		stringify.StructField("id", s.ID()),
		stringify.StructField("address", s.address),
		stringify.StructField("balance", s.balance),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &AliasLockedOutput{}
