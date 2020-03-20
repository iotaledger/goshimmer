package transferoutput

import (
	"fmt"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance"
	transferId "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
)

// Output represents the output of a transfer and contains the balances and the identifiers for this output.
type Output struct {
	address    address.Address
	transferId transferId.Id
	balances   []*coloredbalance.ColoredBalance

	objectstorage.StorableObjectFlags
	storageKey []byte
}

// New creates a transfer output that contains the balances and identifiers of a successful transfer.
func New(address address.Address, transferId transferId.Id, balances []*coloredbalance.ColoredBalance) *Output {
	return &Output{
		address:    address,
		transferId: transferId,
		balances:   balances,

		storageKey: marshalutil.New().WriteBytes(address.Bytes()).WriteBytes(transferId.Bytes()).Bytes(),
	}
}

// FromStorage get's called when we restore a Output from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalBinary (by the ObjectStorage).
func FromStorage(keyBytes []byte) objectstorage.StorableObject {
	return &Output{
		storageKey: marshalutil.New(keyBytes).Bytes(true),
	}
}

// Address returns the address that this output belongs to.
func (transferOutput *Output) Address() address.Address {
	return transferOutput.address
}

// TransferId returns the transfer id, that created this output.
func (transferOutput *Output) TransferId() transferId.Id {
	return transferOutput.transferId
}

// Balances returns the colored balances (color + balance) that this output contains.
func (transferOutput *Output) Balances() []*coloredbalance.ColoredBalance {
	return transferOutput.balances
}

// MarshalBinary marshals the balances into a sequence of bytes - the address and transferId are stored inside the key
// and are ignored here.
func (transferOutput *Output) MarshalBinary() (data []byte, err error) {
	// determine amount of balances in the output
	balanceCount := len(transferOutput.balances)

	// initialize helper
	marshalUtil := marshalutil.New(4 + balanceCount*coloredbalance.Length)

	// marshal the amount of balances
	marshalUtil.WriteUint32(uint32(balanceCount))

	// marshal balances
	for _, balance := range transferOutput.balances {
		marshalUtil.WriteBytes(balance.Bytes())
	}

	return
}

// UnmarshalBinary restores a Output from a serialized version in the ObjectStorage with parts of the object
// being stored in its key rather than the content of the database to reduce storage requirements.
func (transferOutput *Output) UnmarshalBinary(data []byte) (err error) {
	// check if the storageKey has been set
	if transferOutput.storageKey == nil {
		return fmt.Errorf("missing storageKey when trying to unmarshal Output (it contains part of the information)")
	}

	// parse information from storageKey
	storageKeyUnmarshaler := marshalutil.New(transferOutput.storageKey)
	transferOutput.address, err = address.Parse(storageKeyUnmarshaler)
	if err != nil {
		return
	}
	transferOutput.transferId, err = transferId.Parse(storageKeyUnmarshaler)
	if err != nil {
		return
	}

	// parse information from content bytes
	contentUnmarshaler := marshalutil.New(data)
	balanceCount, err := contentUnmarshaler.ReadUint32()
	if err != nil {
		return
	}
	transferOutput.balances = make([]*coloredbalance.ColoredBalance, balanceCount)
	for i := uint32(0); i < balanceCount; i++ {
		transferOutput.balances[i], err = coloredbalance.Parse(contentUnmarshaler)
		if err != nil {
			return
		}
	}

	return
}

// Update is disabled and panics if it ever gets called - it is required to match StorableObject interface.
func (transferOutput *Output) Update(other objectstorage.StorableObject) {
	panic("this object should never be updated")
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (transferOutput *Output) GetStorageKey() []byte {
	return transferOutput.storageKey
}

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Output{}
