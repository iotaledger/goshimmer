package payload

import (
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// GenericDataPayloadType is the Type of a generic GenericDataPayload.
var GenericDataPayloadType = NewType(0, "GenericDataPayloadType", GenericDataPayloadUnmarshaler)

// GenericDataPayloadUnmarshaler is the UnmarshalerFunc of the GenericDataPayload which is also used as a unmarshaler for unknown Types.
func GenericDataPayloadUnmarshaler(data []byte) (Payload, error) {
	return GenericDataPayloadFromMarshalUtil(marshalutil.New(data))
}

// GenericDataPayload represents a payload which just contains a blob of data.
type GenericDataPayload struct {
	payloadType Type
	data        []byte
}

// NewGenericDataPayload creates new GenericDataPayload.
func NewGenericDataPayload(data []byte) *GenericDataPayload {
	return &GenericDataPayload{
		payloadType: GenericDataPayloadType,
		data:        data,
	}
}

// GenericDataPayloadFromBytes unmarshals a GenericDataPayload from a sequence of bytes.
func GenericDataPayloadFromBytes(bytes []byte) (genericDataPayload *GenericDataPayload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if genericDataPayload, err = GenericDataPayloadFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse GenericDataPayload from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// GenericDataPayloadFromMarshalUtil unmarshals a GenericDataPayload using a MarshalUtil (for easier unmarshaling).
func GenericDataPayloadFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (genericDataPayload *GenericDataPayload, err error) {
	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse payload size (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	genericDataPayload = &GenericDataPayload{}
	if genericDataPayload.payloadType, err = TypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Type from MarshalUtil: %w", err)
		return
	}
	if genericDataPayload.data, err = marshalUtil.ReadBytes(int(payloadSize) - TypeLength); err != nil {
		err = xerrors.Errorf("failed to parse data (%v): %w", err, cerrors.ErrParseBytesFailed)
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
	return g.data
}

// Bytes returns a marshaled version of the Payload.
func (g *GenericDataPayload) Bytes() []byte {
	return marshalutil.New().
		WriteUint32(TypeLength + uint32(len(g.data))).
		WriteBytes(g.Type().Bytes()).
		WriteBytes(g.Blob()).
		Bytes()
}

// String returns a human readable version of the Payload.
func (g *GenericDataPayload) String() string {
	return stringify.Struct("GenericDataPayload",
		stringify.StructField("type", g.Type()),
		stringify.StructField("blob", g.Blob()),
	)
}
