package epochs

import (
	"fmt"
	"math"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region EpochID /////////////////////////////////////////////////////////////////////////////////////////////////

// ID is the Epoch's ID.
type ID uint64

// IDFromBytes unmarshals an ID from a sequence of bytes.
func IDFromBytes(bytes []byte) (id ID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if id, err = IDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IDFromMarshalUtil unmarshals an ID using a MarshalUtil (for easier unmarshaling).
func IDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (id ID, err error) {
	untypedID, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse ID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	id = ID(untypedID)
	return
}

// Bytes returns a marshaled version of the ID.
func (i ID) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint64Size).WriteUint64(uint64(i)).Bytes()
}

// String returns a human readable version of the ID.
func (i ID) String() string {
	return fmt.Sprintf("EpochID(%X)", uint64(i))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Epoch /////////////////////////////////////////////////////////////////////////////////////////////////////

// Epoch is an universal time interval that allows to track active nodes and their mana.
type Epoch struct {
	objectstorage.StorableObjectFlags

	id            ID
	mana          map[identity.ID]float64
	totalMana     float64
	manaRetrieved bool

	manaMutex sync.RWMutex
}

// NewEpoch is the constructor for an Epoch.
func NewEpoch(id ID) *Epoch {
	return &Epoch{
		id:   id,
		mana: make(map[identity.ID]float64),
	}
}

// EpochFromBytes parses the given bytes into an Epoch.
func EpochFromBytes(bytes []byte) (result *Epoch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = EpochFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// EpochFromMarshalUtil parses a new Epoch from the given marshal util.
func EpochFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Epoch, err error) {
	result = &Epoch{
		mana: make(map[identity.ID]float64),
	}
	if result.id, err = IDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse EpochID from MarshalUtil: %w", err)
		return
	}

	var nodesCount uint32
	if nodesCount, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse nodes count from MarshalUtil: %w", err)
		return
	}

	for i := 0; i < int(nodesCount); i++ {
		var nodeID identity.ID
		if nodeID, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse nodeID from MarshalUtil: %w", err)
			return
		}

		var mana uint64
		if mana, err = marshalUtil.ReadUint64(); err != nil {
			err = xerrors.Errorf("failed to parse mana value from MarshalUtil: %w", err)
			return
		}

		result.mana[nodeID] = math.Float64frombits(mana)
	}

	if result.totalMana, err = marshalUtil.ReadFloat64(); err != nil {
		err = xerrors.Errorf("failed to parse total mana value from MarshalUtil: %w", err)
		return
	}

	return result, nil
}

// EpochFromObjectStorage is the factory method for Epoch stored in the ObjectStorage.
func EpochFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = EpochFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse Epoch from bytes: %w", err)
		return
	}

	return
}

// ID returns the Epoch's ID.
func (e *Epoch) ID() ID {
	return e.id
}

// AddNode adds an active node to this Epoch.
func (e *Epoch) AddNode(id identity.ID) {
	e.manaMutex.Lock()
	defer e.manaMutex.Unlock()

	e.mana[id] = 0

	e.SetModified()
}

// ManaRetrieved describes whether the active consensus mana was already retrieved. Retrieving/setting the consensus
// mana needs to be done only once per Epoch.
func (e *Epoch) ManaRetrieved() bool {
	e.manaMutex.RLock()
	defer e.manaMutex.RUnlock()

	return e.manaRetrieved
}

// SetMana updates the active nodes from a consensus mana vector. If forceOverride is true, the active consensus mana
// will be copied from the given consensus mana vector.
func (e *Epoch) SetMana(consensusMana map[identity.ID]float64, forceOverride ...bool) {
	var force bool
	if len(forceOverride) > 0 {
		force = forceOverride[0]
	}

	e.manaMutex.Lock()
	defer func() {
		e.manaRetrieved = true
		e.SetModified()
		e.manaMutex.Unlock()
	}()

	if force {
		e.totalMana = 0
		for nodeID, mana := range consensusMana {
			e.mana[nodeID] = mana
			e.totalMana += mana
		}
		return
	}

	for nodeID := range e.mana {
		e.mana[nodeID] = consensusMana[nodeID]
		e.totalMana += consensusMana[nodeID]
	}

	// clean up nodes that do not have any mana
	for nodeID := range e.mana {
		if e.mana[nodeID] == 0 {
			delete(e.mana, nodeID)
		}
	}
}

// Mana returns a map of active nodes and their consensus mana within the Epoch.
func (e *Epoch) Mana() (mana map[identity.ID]float64) {
	e.manaMutex.RLock()
	defer e.manaMutex.RUnlock()

	mana = make(map[identity.ID]float64)
	for nodeID, m := range e.mana {
		mana[nodeID] = m
	}
	return
}

// TotalMana returns the total active consensus mana within this Epoch.
func (e *Epoch) TotalMana() float64 {
	e.manaMutex.RLock()
	defer e.manaMutex.RUnlock()

	return e.totalMana
}

// Bytes returns a marshaled version of the whole Epoch object.
func (e *Epoch) Bytes() []byte {
	return byteutils.ConcatBytes(e.ObjectStorageKey(), e.ObjectStorageValue())
}

// ObjectStorageKey returns the key of the stored Epoch object.
// This returns the bytes of the ID.
func (e *Epoch) ObjectStorageKey() []byte {
	return e.id.Bytes()
}

// ObjectStorageValue returns the value of the stored Epoch object.
func (e *Epoch) ObjectStorageValue() []byte {
	marshalUtil := marshalutil.New()

	e.manaMutex.RLock()
	defer e.manaMutex.RUnlock()

	marshalUtil.WriteUint32(uint32(len(e.mana)))
	for nodeID, mana := range e.mana {
		marshalUtil.Write(nodeID)
		marshalUtil.WriteUint64(math.Float64bits(mana))
	}
	marshalUtil.WriteFloat64(e.totalMana)

	return marshalUtil.Bytes()
}

// Update updates the Epoch.
// This should never happen and will panic if attempted.
func (e *Epoch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// String returns a human readable version of the Epoch.
func (e *Epoch) String() string {
	builder := stringify.StructBuilder("Epoch", stringify.StructField("ID", e.id))

	e.manaMutex.RLock()
	for nodeID, mana := range e.mana {
		builder.AddField(stringify.StructField(nodeID.String(), fmt.Sprintf("%.2f", mana)))
	}
	e.manaMutex.RUnlock()
	builder.AddField(stringify.StructField("TotalMana", fmt.Sprintf("%.2f", e.TotalMana())))
	return builder.String()
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ objectstorage.StorableObject = &Epoch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedEpoch ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedEpoch is a wrapper for a stored cached object representing an Epoch.
type CachedEpoch struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the CachedEpoch into the underlying Epoch.
// If stored object cannot be cast into an Epoch or has been deleted, it returns nil.
func (c *CachedEpoch) Unwrap() *Epoch {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Epoch)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume consumes the CachedEpoch.
// It releases the object when the callback is done.
// It returns true if the callback was called.
func (c *CachedEpoch) Consume(consumer func(epoch *Epoch), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Epoch))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
