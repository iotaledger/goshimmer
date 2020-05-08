package utxodag

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
)

// ValueObjectBooking stores the branch of a ValueObject that it is booked in.
type ValueObjectBooking struct {
	objectstorage.StorableObjectFlags

	id            payload.ID
	branchID      branchmanager.BranchID
	branchIDMutex sync.RWMutex
}

// NewValueObjectBooking is the constructor of the ValueObjectBooking entitiy.
func NewValueObjectBooking(id payload.ID) *ValueObjectBooking {
	return &ValueObjectBooking{
		id: id,
	}
}

// ValueObjectBookingFromStorageKey is a factory method that creates a new ValueObjectBooking instance from a storage
// key of the objectstorage. It is used by the objectstorage, to create new instances of this entity.
func ValueObjectBookingFromStorageKey(key []byte, optionalTargetObject ...*ValueObjectBooking) (result *ValueObjectBooking, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &ValueObjectBooking{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ValueObjectBookingFromStorageKey")
	}

	// parse information
	marshalUtil := marshalutil.New(key)
	result.id, err = payload.ParseID(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ValueObjectBookingFromBytes unmarshals a ValueObjectBooking from a sequence of bytes.
func ValueObjectBookingFromBytes(bytes []byte, optionalTargetObject ...*ValueObjectBooking) (result *ValueObjectBooking, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseValueObjectBooking(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseValueObjectBooking unmarshals a ValueObjectBooking using the given marshalUtil (for easier
// marshaling/unmarshaling).
func ParseValueObjectBooking(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*ValueObjectBooking) (result *ValueObjectBooking, err error) {
	parsedObject, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ValueObjectBookingFromStorageKey(data, optionalTargetObject...)
	})
	if err != nil {
		return
	}

	result = parsedObject.(*ValueObjectBooking)
	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// ID returns the identifier of the ValueObject that this metadata belongs to.
func (valueObjectBooking *ValueObjectBooking) ID() payload.ID {
	return valueObjectBooking.id
}

// BranchID returns the identifier of the Branch that this ValueObject belongs to.
func (valueObjectBooking *ValueObjectBooking) BranchID() branchmanager.BranchID {
	valueObjectBooking.branchIDMutex.RLock()
	defer valueObjectBooking.branchIDMutex.RUnlock()

	return valueObjectBooking.branchID
}

// SetBranchID is the setter for the BranchID property. It returns true of the value has been modified.
func (valueObjectBooking *ValueObjectBooking) SetBranchID(branchID branchmanager.BranchID) (modified bool) {
	valueObjectBooking.branchIDMutex.RLock()
	if valueObjectBooking.branchID == branchID {
		valueObjectBooking.branchIDMutex.RUnlock()

		return
	}

	valueObjectBooking.branchIDMutex.RUnlock()
	valueObjectBooking.branchIDMutex.Lock()
	defer valueObjectBooking.branchIDMutex.Unlock()

	if valueObjectBooking.branchID == branchID {
		return
	}

	valueObjectBooking.branchID = branchID
	valueObjectBooking.SetModified()
	modified = true

	return
}

// Update panics when it is called. It is required for the StorableObject interface but the feature to "replace" an
// object is disabled in favor of setters.
func (valueObjectBooking *ValueObjectBooking) Update(other objectstorage.StorableObject) {
	panic("updates are not allowed - use the setters")
}

// ObjectStorageKey returns the bytes that are used as a key when storing the Branch in an objectstorage.
func (valueObjectBooking *ValueObjectBooking) ObjectStorageKey() []byte {
	return valueObjectBooking.id.Bytes()
}

// ObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a marshaled
// ObjectStorageValue.
func (valueObjectBooking *ValueObjectBooking) ObjectStorageValue() []byte {
	return valueObjectBooking.BranchID().Bytes()
}

// UnmarshalObjectStorageValue unmarshals the bytes that are stored in the value of the objectstorage.
func (valueObjectBooking *ValueObjectBooking) UnmarshalObjectStorageValue(valueBytes []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(valueBytes)
	if valueObjectBooking.branchID, err = branchmanager.ParseBranchID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// interface contract
var _ objectstorage.StorableObject = &ValueObjectBooking{}
