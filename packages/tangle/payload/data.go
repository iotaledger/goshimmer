package payload

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(GenericDataPayload{}, serix.TypeSettings{}.WithObjectType(uint32(new(GenericDataPayload).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering GenericDataPayload type settings: %w", err))
	}

	err = serix.DefaultAPI.RegisterInterfaceObjects((*Payload)(nil), new(GenericDataPayload))
	if err != nil {
		panic(fmt.Errorf("error registering GenericDataPayload as Payload interface: %w", err))
	}
}

// GenericDataPayloadType is the Type of a generic GenericDataPayload.
var GenericDataPayloadType = NewType(0, "GenericDataPayloadType")

// GenericDataPayload represents a payload which just contains a blob of data.
type GenericDataPayload struct {
	genericDataPayloadInner `serix:"0"`
}
type genericDataPayloadInner struct {
	payloadType Type
	Data        []byte `serix:"0,lengthPrefixType=uint32"`
}

// NewGenericDataPayload creates new GenericDataPayload.
func NewGenericDataPayload(data []byte) *GenericDataPayload {
	return &GenericDataPayload{genericDataPayloadInner{
		payloadType: GenericDataPayloadType,
		Data:        data,
	}}
}

// GenericDataPayloadFromBytes unmarshals a GenericDataPayload from a sequence of bytes.
func GenericDataPayloadFromBytes(bytes []byte) (genericDataPayload *GenericDataPayload, consumedBytes int, err error) {
	genericDataPayload = new(GenericDataPayload)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, genericDataPayload, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse GenericDataPayload: %w", err)
		return
	}

	return
}

// Type returns the Type of the Payload.
func (g *GenericDataPayload) Type() Type {
	return g.payloadType
}

// Blob returns the contained data of the GenericDataPayload (without its type and size headers).
func (g *GenericDataPayload) Blob() []byte {
	return g.Data
}

// Bytes returns a marshaled version of the Payload.
func (g *GenericDataPayload) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), g, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human readable version of the Payload.
func (g *GenericDataPayload) String() string {
	return stringify.Struct("GenericDataPayload",
		stringify.StructField("type", g.Type()),
		stringify.StructField("blob", g.Blob()),
	)
}
