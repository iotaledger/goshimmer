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
	"time"
)

// ExtendedLockedOutput is an Extension of SigLockedColoredOutput.
// In addition it has optional:
// - fallback address and timeout
// - can be unlocked by AliasReferencedUnlockBlock
// - can't be time locked until deadline
type ExtendedLockedOutput struct {
	id       OutputID
	idMutex  sync.RWMutex
	balances *ColoredBalances
	address  Address // any address type

	fallbackAddress  Address // if nil, fallback action not set
	fallbackDeadline uint32  // fallback deadline in Unix seconds. The deadline is calculated relative to the tx timestamo

	timelock uint32 // deadlne since when output can be unlocked

	objectstorage.StorableObjectFlags
}

// ExtendedLockedOutput is the constructor for a ExtendedLockedOutput.
func NewExtendedLockedOutput(balances map[Color]uint64, address Address) *ExtendedLockedOutput {
	return &ExtendedLockedOutput{
		balances: NewColoredBalances(balances),
		address:  address,
	}
}

func (o *ExtendedLockedOutput) WithFallbackOptions(addr Address, deadline uint32) *ExtendedLockedOutput {
	o.fallbackAddress = addr
	o.fallbackDeadline = deadline
	return o
}

func (o *ExtendedLockedOutput) WithTimeLock(timelock uint32) *ExtendedLockedOutput {
	o.timelock = timelock
	return o
}

// ExtendedOutputFromBytes unmarshals a ExtendedLockedOutput from a sequence of bytes.
func ExtendedOutputFromBytes(bytes []byte) (output *ExtendedLockedOutput, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if output, err = ExtendedOutputFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ExtendedLockedOutput from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ExtendedOutputFromMarshalUtil unmarshals a ExtendedLockedOutput using a MarshalUtil (for easier unmarshaling).
func ExtendedOutputFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (output *ExtendedLockedOutput, err error) {
	outputType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse OutputType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if OutputType(outputType) != ExtendedLockedOutputType {
		err = xerrors.Errorf("invalid OutputType (%X): %w", outputType, cerrors.ErrParseBytesFailed)
		return
	}

	output = &ExtendedLockedOutput{}
	if output.balances, err = ColoredBalancesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ColoredBalances: %w", err)
		return
	}
	if output.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Address (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if output.timelock, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse timelock (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	var fallbackExists bool
	fallbackExists, err = marshalUtil.ReadBool()
	if err != nil || !fallbackExists {
		return
	}
	if output.fallbackAddress, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse fallbackAddress (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if output.fallbackDeadline, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse fallbackTimeout (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// ID returns the identifier of the Output that is used to address the Output in the UTXODAG.
func (o *ExtendedLockedOutput) ID() OutputID {
	o.idMutex.RLock()
	defer o.idMutex.RUnlock()

	return o.id
}

// SetID allows to set the identifier of the Output. We offer a setter for the property since Outputs that are
// created to become part of a transaction usually do not have an identifier, yet as their identifier depends on
// the TransactionID that is only determinable after the Transaction has been fully constructed. The ID is therefore
// only accessed when the Output is supposed to be persisted by the node.
func (o *ExtendedLockedOutput) SetID(outputID OutputID) Output {
	o.idMutex.Lock()
	defer o.idMutex.Unlock()

	o.id = outputID

	return o
}

// Type returns the type of the Output which allows us to generically handle Outputs of different types.
func (o *ExtendedLockedOutput) Type() OutputType {
	return ExtendedLockedOutputType
}

// Balances returns the funds that are associated with the Output.
func (o *ExtendedLockedOutput) Balances() *ColoredBalances {
	return o.balances
}

// UnlockValid determines if the given Transaction and the corresponding UnlockBlock are allowed to spend the Output.
func (o *ExtendedLockedOutput) UnlockValid(tx *Transaction, unlockBlock UnlockBlock, inputs []Output) (unlockValid bool, err error) {
	if tx.Essence().Timestamp().Before(time.Unix(int64(o.timelock), 0)) {
		return false, nil
	}
	var addr Address
	switch blk := unlockBlock.(type) {
	case *SignatureUnlockBlock:
		if o.fallbackAddress == nil {
			addr = o.address
		} else {
			if tx.Essence().Timestamp().Before(time.Unix(int64(o.fallbackDeadline), 0)) {
				addr = o.address
			} else {
				addr = o.fallbackAddress
			}
		}
		unlockValid = blk.AddressSignatureValid(addr, tx.Essence().Bytes())

	case *AliasReferencedUnlockBlock:
		var ok bool
		if o.fallbackAddress == nil {
			addr = o.address
		} else {
			if tx.Essence().Timestamp().Before(time.Unix(int64(o.fallbackDeadline), 0)) {
				addr = o.address
			} else {
				addr = o.fallbackAddress
			}
		}
		aliasAddress, ok := addr.(*AliasAddress)
		if !ok {
			return false, nil
		}
		unlockValid, _ = UnlockedAliasStateByIndex(tx, aliasAddress, blk.ReferencedIndex(), inputs)

	default:
		err = xerrors.Errorf("UnlockBlock does not match expected OutputType: %w", cerrors.ErrParseBytesFailed)
	}
	return
}

// Address returns the Address that the Output is associated to.
func (o *ExtendedLockedOutput) Address() Address {
	return o.address
}

// Input returns an Input that references the Output.
func (o *ExtendedLockedOutput) Input() Input {
	if o.ID() == EmptyOutputID {
		panic("Outputs that haven't been assigned an ID, yet cannot be converted to an Input")
	}

	return NewUTXOInput(o.ID())
}

// Clone creates a copy of the Output.
func (o *ExtendedLockedOutput) Clone() Output {
	return &ExtendedLockedOutput{
		id:       o.id,
		balances: o.balances.Clone(),
		address:  o.address.Clone(),
	}
}

// UpdateMintingColor replaces the ColorMint in the balances of the Output with the hash of the OutputID. It returns a
// copy of the original Output with the modified balances.
func (o *ExtendedLockedOutput) UpdateMintingColor() (updatedOutput *ExtendedLockedOutput) {
	coloredBalances := o.Balances().Map()
	if mintedCoins, mintedCoinsExist := coloredBalances[ColorMint]; mintedCoinsExist {
		delete(coloredBalances, ColorMint)
		coloredBalances[Color(blake2b.Sum256(o.ID().Bytes()))] = mintedCoins
	}
	updatedOutput = NewExtendedLockedOutput(coloredBalances, o.Address())
	updatedOutput.SetID(o.ID())

	return
}

// Bytes returns a marshaled version of the Output.
func (o *ExtendedLockedOutput) Bytes() []byte {
	return o.ObjectStorageValue()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *ExtendedLockedOutput) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *ExtendedLockedOutput) ObjectStorageKey() []byte {
	return o.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (o *ExtendedLockedOutput) ObjectStorageValue() []byte {
	ret := marshalutil.New().
		WriteByte(byte(ExtendedLockedOutputType)).
		WriteBytes(o.balances.Bytes()).
		WriteBytes(o.address.Bytes()).
		WriteUint32(o.timelock).
		WriteBool(o.fallbackAddress != nil)

	if o.fallbackAddress != nil {
		ret.WriteBytes(o.fallbackAddress.Bytes()).
			WriteUint32(o.fallbackDeadline)
	}
	return ret.Bytes()
}

// Compare offers a comparator for Outputs which returns -1 if the other Output is bigger, 1 if it is smaller and 0 if
// they are the same.
func (o *ExtendedLockedOutput) Compare(other Output) int {
	return bytes.Compare(o.Bytes(), other.Bytes())
}

// String returns a human readable version of the Output.
func (o *ExtendedLockedOutput) String() string {
	return stringify.Struct("ExtendedLockedOutput",
		stringify.StructField("id", o.ID()),
		stringify.StructField("address", o.address),
		stringify.StructField("balances", o.balances),
	)
}

// code contract (make sure the type implements all required methods)
var _ Output = &ExtendedLockedOutput{}
