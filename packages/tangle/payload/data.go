package payload

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// DataType is the Type of a generic Data payload.
var DataType = NewType(0, "Data", DataUnmarshaler)

// DataUnmarshaler is the UnmarshalerFunc of the Data payload which is also used as a unmarshaler for unknown Types.
func DataUnmarshaler(data []byte) (Payload, error) {
	return DataFromMarshalUtil(marshalutil.New(data))
}

// Data represents a payload which just contains a blob of data.
type Data struct {
	payloadType Type
	data        []byte
}

// NewData creates new Data payload.
func NewData(data []byte) *Data {
	return &Data{
		payloadType: DataType,
		data:        data,
	}
}

// DataFromBytes unmarshals a Data payload from a sequence of bytes.
func DataFromBytes(bytes []byte) (result *Data, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if result, err = DataFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Data from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// DataFromMarshalUtil unmarshals a Data payload using a MarshalUtil (for easier unmarshaling).
func DataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Data, err error) {
	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse payload size (%v): %w", err, ErrParseBytesFailed)
		return
	}

	result = &Data{}
	if result.payloadType, err = TypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Type from MarshalUtil: %w", err)
		return
	}
	if result.data, err = marshalUtil.ReadBytes(int(payloadSize)); err != nil {
		err = xerrors.Errorf("failed to parse data (%v): %w", err, ErrParseBytesFailed)
		return
	}

	return
}

// Type returns the Type of the Payload.
func (d *Data) Type() Type {
	return d.payloadType
}

// Blob returns the contained data of the Data payload (without its type and size headers).
func (d *Data) Blob() []byte {
	return d.data
}

// Bytes returns a marshaled version of the Payload.
func (d *Data) Bytes() []byte {
	return marshalutil.New().
		WriteUint32(uint32(len(d.data))).
		WriteBytes(d.Type().Bytes()).
		WriteBytes(d.Blob()).
		Bytes()
}

// String returns a human readable version of the Payload.
func (d *Data) String() string {
	return stringify.Struct("Data",
		stringify.StructField("type", d.Type()),
		stringify.StructField("blob", d.Blob()),
	)
}
