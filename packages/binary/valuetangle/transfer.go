package valuetangle

import (
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type Transfer struct {
	objectstorage.StorableObjectFlags

	id     *TransferId
	inputs *TransferInputs
	bytes  []byte

	idMutex    sync.RWMutex
	bytesMutex sync.RWMutex
}

func NewTransfer(inputs *TransferInputs) *Transfer {
	return &Transfer{
		inputs: inputs,
	}
}

func FromStorage(key []byte) *Transfer {
	id := NewTransferId(key)

	return &Transfer{
		id: &id,
	}
}

func (transfer *Transfer) GetId() TransferId {
	// acquire lock for reading id
	transfer.idMutex.RLock()

	// return if id has been calculated already
	if transfer.id != nil {
		defer transfer.idMutex.RUnlock()

		return *transfer.id
	}

	// switch to write lock
	transfer.idMutex.RUnlock()
	transfer.idMutex.Lock()
	defer transfer.idMutex.Unlock()

	// return if id has been calculated in the mean time
	if transfer.id != nil {
		return *transfer.id
	}

	// otherwise calculate the id
	bytes, err := transfer.MarshalBinary()
	if err != nil {
		panic(err)
	}
	idBytes := blake2b.Sum256(bytes)
	transferId := NewTransferId(idBytes[:])

	// cache result for later calls
	transfer.id = &transferId

	return transferId
}

func (transfer *Transfer) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

func (transfer *Transfer) GetStorageKey() []byte {
	id := transfer.GetId()

	return id[:]
}

func (transfer *Transfer) MarshalBinary() ([]byte, error) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(transfer.inputs.ToBytes())

	return marshalUtil.Bytes(), nil
}

func (transfer *Transfer) UnmarshalBinary(data []byte) error {
	marshalUtil := marshalutil.New(data)

	parseResult, err := marshalUtil.Parse(func(data []byte) (result interface{}, err error, consumedBytes int) {
		return TransferInputsFromBytes(data)
	})
	if err != nil {
		return err
	}
	transfer.inputs = parseResult.(*TransferInputs)

	return nil
}

// define contracts (ensure that the struct fulfills the corresponding interfaces
var _ objectstorage.StorableObject = &Transfer{}
