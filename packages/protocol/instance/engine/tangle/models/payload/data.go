package payload

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
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
	model.Immutable[GenericDataPayload, *GenericDataPayload, genericDataPayloadInner] `serix:"0"`
}
type genericDataPayloadInner struct {
	Data []byte `serix:"0,lengthPrefixType=uint32"`
}

// NewGenericDataPayload creates new GenericDataPayload.
func NewGenericDataPayload(data []byte) *GenericDataPayload {
	return model.NewImmutable[GenericDataPayload](&genericDataPayloadInner{
		Data: data,
	})
}

// Type returns the Type of the Payload.
func (g *GenericDataPayload) Type() Type {
	return GenericDataPayloadType
}

// Blob returns the contained data of the GenericDataPayload (without its type and size headers).
func (g *GenericDataPayload) Blob() []byte {
	return g.M.Data
}
