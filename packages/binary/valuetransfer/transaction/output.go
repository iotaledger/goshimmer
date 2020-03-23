package transaction

import (
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
)

// Output represents the output of a Transaction and contains the balances and the identifiers for this output.
type Output struct {
	address       address.Address
	transactionId Id
	balances      []*balance.Balance

	objectstorage.StorableObjectFlags
	storageKey []byte
}

// NewOutput creates an Output that contains the balances and identifiers of a Transaction.
func NewOutput(address address.Address, transactionId Id, balances []*balance.Balance) *Output {
	return &Output{
		address:       address,
		transactionId: transactionId,
		balances:      balances,

		storageKey: marshalutil.New().WriteBytes(address.Bytes()).WriteBytes(transactionId.Bytes()).Bytes(),
	}
}

// OutputFromStorage get's called when we restore a Output from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalBinary (by the ObjectStorage).
func OutputFromStorage(keyBytes []byte) objectstorage.StorableObject {
	return &Output{
		storageKey: marshalutil.New(keyBytes).Bytes(true),
	}
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

// MarshalBinary marshals the balances into a sequence of bytes - the address and transaction id are stored inside the key
// and are ignored here.
func (output *Output) MarshalBinary() (data []byte, err error) {
	// determine amount of balances in the output
	balanceCount := len(output.balances)

	// initialize helper
	marshalUtil := marshalutil.New(4 + balanceCount*balance.Length)

	// marshal the amount of balances
	marshalUtil.WriteUint32(uint32(balanceCount))

	// marshal balances
	for _, balance := range output.balances {
		marshalUtil.WriteBytes(balance.Bytes())
	}

	return
}

// UnmarshalBinary restores a Output from a serialized version in the ObjectStorage with parts of the object
// being stored in its key rather than the content of the database to reduce storage requirements.
func (output *Output) UnmarshalBinary(data []byte) (err error) {
	// check if the storageKey has been set
	if output.storageKey == nil {
		return fmt.Errorf("missing storageKey when trying to unmarshal Output (it contains part of the information)")
	}

	// parse information from storageKey
	storageKeyUnmarshaler := marshalutil.New(output.storageKey)
	output.address, err = address.Parse(storageKeyUnmarshaler)
	if err != nil {
		return
	}
	output.transactionId, err = ParseId(storageKeyUnmarshaler)
	if err != nil {
		return
	}

	// parse information from content bytes
	contentUnmarshaler := marshalutil.New(data)
	balanceCount, err := contentUnmarshaler.ReadUint32()
	if err != nil {
		return
	}
	output.balances = make([]*balance.Balance, balanceCount)
	for i := uint32(0); i < balanceCount; i++ {
		output.balances[i], err = balance.Parse(contentUnmarshaler)
		if err != nil {
			return
		}
	}

	return
}

// Update is disabled and panics if it ever gets called - it is required to match StorableObject interface.
func (output *Output) Update(other objectstorage.StorableObject) {
	panic("this object should never be updated")
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (output *Output) GetStorageKey() []byte {
	return output.storageKey
}

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Output{}
