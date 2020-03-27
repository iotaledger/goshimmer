package transaction

import (
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
)

// Output represents the output of a Transaction and contains the balances and the identifiers for this output.
type Output struct {
	address       address.Address
	transactionId Id
	solid         bool
	solidSince    time.Time
	balances      []*balance.Balance

	objectstorage.StorableObjectFlags
	storageKey []byte
}

// NewOutput creates an Output that contains the balances and identifiers of a Transaction.
func NewOutput(address address.Address, transactionId Id, balances []*balance.Balance) *Output {
	return &Output{
		address:       address,
		transactionId: transactionId,
		solid:         false,
		solidSince:    time.Time{},
		balances:      balances,

		storageKey: marshalutil.New().WriteBytes(address.Bytes()).WriteBytes(transactionId.Bytes()).Bytes(),
	}
}

// OutputFromBytes unmarshals an Output object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func OutputFromBytes(bytes []byte, optionalTargetObject ...*Output) (result *Output, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Output{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.address, err = address.Parse(marshalUtil); err != nil {
		return
	}
	if result.transactionId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if result.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if result.solidSince, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	var balanceCount uint32
	if balanceCount, err = marshalUtil.ReadUint32(); err != nil {
		return
	} else {
		result.balances = make([]*balance.Balance, balanceCount)
		for i := uint32(0); i < balanceCount; i++ {
			result.balances[i], err = balance.Parse(marshalUtil)
			if err != nil {
				return
			}
		}
	}
	result.storageKey = marshalutil.New().WriteBytes(result.address.Bytes()).WriteBytes(result.transactionId.Bytes()).Bytes()
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OutputFromStorageKey get's called when we restore a Output from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalObjectStorageValue (by the ObjectStorage).
func OutputFromStorageKey(keyBytes []byte) (objectstorage.StorableObject, error) {
	return &Output{
		storageKey: keyBytes[:OutputIdLength],
	}, nil
}

// Address returns the address that this output belongs to.
func (output *Output) Address() address.Address {
	return output.address
}

// TransactionId returns the id of the Transaction, that created this output.
func (output *Output) TransactionId() Id {
	return output.transactionId
}

// Balances returns the colored balances (color + balance) that this output contains.
func (output *Output) Balances() []*balance.Balance {
	return output.balances
}

// ObjectStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (output *Output) ObjectStorageKey() []byte {
	return marshalutil.New(OutputIdLength).
		WriteBytes(output.address.Bytes()).
		WriteBytes(output.transactionId.Bytes()).
		Bytes()
}

// ObjectStorageValue marshals the balances into a sequence of bytes - the address and transaction id are stored inside the key
// and are ignored here.
func (output *Output) ObjectStorageValue() (data []byte) {
	// determine amount of balances in the output
	balanceCount := len(output.balances)

	// initialize helper
	marshalUtil := marshalutil.New(4 + balanceCount*balance.Length)
	marshalUtil.WriteBool(output.solid)
	marshalUtil.WriteTime(output.solidSince)
	marshalUtil.WriteUint32(uint32(balanceCount))
	for _, balanceToMarshal := range output.balances {
		marshalUtil.WriteBytes(balanceToMarshal.Bytes())
	}

	return
}

// UnmarshalObjectStorageValue restores a Output from a serialized version in the ObjectStorage with parts of the object
// being stored in its key rather than the content of the database to reduce storage requirements.
func (output *Output) UnmarshalObjectStorageValue(data []byte) (err error) {
	_, err, _ = OutputFromBytes(marshalutil.New(output.storageKey).WriteBytes(data).Bytes(), output)

	return
}

// Update is disabled and panics if it ever gets called - it is required to match StorableObject interface.
func (output *Output) Update(other objectstorage.StorableObject) {
	panic("this object should never be updated")
}

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Output{}
