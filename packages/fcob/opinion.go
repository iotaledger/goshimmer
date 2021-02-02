package fcob

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

// LevelOfKnowledge defines the Level Of Knowledge type.
type LevelOfKnowledge = uint8

// The different levels of knowledge.
const (
	// One implies that voting is required.
	One LevelOfKnowledge = iota + 1

	// Two implies that we have finalized our opinion but we can still reply to eventual queries.
	Two

	// Three implies that we have finalized our opinion and we do not reply to eventual queries.
	Three
)

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

type Opinion struct {
	transactionID    ledgerstate.TransactionID
	Liked            bool
	LevelOfKnowledge LevelOfKnowledge

	objectstorage.StorableObjectFlags
}

func (o *Opinion) Bytes() []byte {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

func (o *Opinion) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

func (o *Opinion) ObjectStorageKey() []byte {
	return o.transactionID.Bytes()
}

func (o *Opinion) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteBool(o.Liked).
		WriteUint8(o.LevelOfKnowledge).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &Opinion{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOpinion ////////////////////////////////////////////////////////////////////////////////////////////////

// CachedOpinion is a wrapper for the generic CachedObject returned by the object storage that overrides the accessor
// methods with a type-casted one.
type CachedOpinion struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedOpinion) Retain() *CachedOpinion {
	return &CachedOpinion{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedOpinion) Unwrap() *Opinion {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Opinion)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedOpinion) Consume(consumer func(opinion *Opinion), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Opinion))
	}, forceRelease...)
}

// String returns a human readable version of the CachedOpinion.
func (c *CachedOpinion) String() string {
	return stringify.Struct("CachedOpinion",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
