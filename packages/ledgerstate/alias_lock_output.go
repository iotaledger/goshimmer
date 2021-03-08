package ledgerstate

import (
	"bytes"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
	"sync"
)

// ExtendedOutput is an Extension of SigLockedColoredOutput.
// In addition it has optional:
// - fallback address and timeout
// - can be unlocked by AliasReferencedUnlockBlock
type ExtendedOutput struct {
	id       OutputID
	idMutex  sync.RWMutex
	balances *ColoredBalances
	address  Address

	objectstorage.StorableObjectFlags
}

// NewExtendedOutput is the constructor for a ExtendedOutput.
func NewExtendedOutput(balances *ColoredBalances, address Address) *ExtendedOutput {
	return &ExtendedOutput{
		balances: balances,
		address:  address,
	}
}

// ExtendedOutputFromBytes unmarshals a ExtendedOutput from a sequence of bytes.
func ExtendedOutputFromBytes(bytes []byte) (output *ExtendedOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = ExtendedOutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ExtendedOutput from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ExtendedOutputFromMarshalUtil unmarshals a ExtendedOutput using a MarshalUtil (for easier unmarshaling).
func ExtendedOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *ExtendedOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != ExtendedOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	output = &ExtendedOutput{}
	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (s *ExtendedOutput) ID() OutputID {
	s.idMutex.RLock()
	defer s.idMutex.RUnlock()

	return s.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (s *ExtendedOutput) SetID(outputID OutputID) Output {
	s.idMutex.Lock()
	defer s.idMutex.Unlock()

	s.id = outputID

	return s
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (s *ExtendedOutput) Type() OutputType {
	return ExtendedOutputType
}

// Balances returns the funds that are associated with the Output.
func (s *ExtendedOutput) Balances() *ColoredBalances {
	return s.balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (s *ExtendedOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
	signatureUnlockBlock, correctType := unlockBlock.(*SignatureUnlockBlock)
	if !correctType {
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", cerrors.ErrParseBytesFailed)
		return
	}

	unlockValid = signatureUnlockBlock.AddressSignatureValid(s.address, tx.Essence().Bytes())

	return
}

// Address returns the Address that the Output is associated to.
func (s *ExtendedOutput) Address() Address {
	return s.address
}

// Input returns an Input that references the Output.
func (s *ExtendedOutput) Input() Input {
	if s.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(s.ID())
}

// Clone creates a copy of the Output.
func (s *ExtendedOutput) Clone() Output {
	return &ExtendedOutput{
		id:       s.id,
		balances: s.balances.Clone(),
		address:  s.address.Clone(),
	}
}

// UpdateMintingColor replaces the ColorMint in the balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified balances.
func (s *ExtendedOutput) UpdateMintingColor() (updatedOutput *ExtendedOutput) {
	coloredBalances := s.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(s.ID().Bytes()))] = mintedCoins
	}
	updatedOutput = NewExtendedOutput(NewColoredBalances(coloredBalances), s.Address())
	updatedOutput.SetID(s.ID())

	return
}

// Bytes returns a marshaled version of the Output.
func (s *ExtendedOutput) Bytes() []byte {
	return s.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *ExtendedOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *ExtendedOutput) ObjectStorageKey() []byte {
	return s.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (s *ExtendedOutput) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(ExtendedOutputType)).
		WriteBytes(s.balances.Bytes()).
		WriteBytes(s.address.Bytes()).
		Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (s *ExtendedOutput) Compare(other Output) int {
	return bytes.Compare(s.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (s *ExtendedOutput) String() string {
	return stringify.Struct("ExtendedOutput",
		stringify.StructField("id", s.ID()),
		stringify.StructField("address", s.address),
		stringify.StructField("balances", s.balances),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &ExtendedOutput{}
