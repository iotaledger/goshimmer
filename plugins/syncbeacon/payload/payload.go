package payload

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// ObjectName defines the name of the syncbeacon object.
	ObjectName = "syncbeacon"
)

// Type is the type of the syncbeacon payload.
var Type = payload.Type(200)

// Payload represents the syncbeacon payload
type Payload struct {
	payloadType payload.Type
	sentTime    int64
}

// NewSyncBeaconPayload creates a new syncbeacon payload
func NewSyncBeaconPayload(sentTime int64) *Payload {
	return &Payload{
		payloadType: Type,
		sentTime:    sentTime,
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Payload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read data
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	_, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	result.sentTime, err = marshalUtil.ReadInt64()
	if err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return p.payloadType
}

// SentTime returns the time that payload was sent.
func (p *Payload) SentTime() int64 {
	return p.sentTime
}

// Bytes marshals the syncbeacon payload into a sequence of bytes.
func (p *Payload) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()
	objectLength := marshalutil.INT64_SIZE

	// marshal the p specific information
	marshalUtil.WriteUint32(Type)
	marshalUtil.WriteUint32(uint32(objectLength))
	marshalUtil.WriteInt64(p.sentTime)

	// return result
	return marshalUtil.Bytes()
}

// Unmarshal unmarshals a given slice of bytes and fills the object.
func (p *Payload) Unmarshal(data []byte) (err error) {
	_, err, _ = FromBytes(data, p)

	return
}

// String returns a human readable version of syncbeacon payload (for debug purposes).
func (p *Payload) String() string {
	return stringify.Struct("syncBeaconPayload",
		stringify.StructField("sentTime", p.sentTime))
}

// IsSyncBeaconPayload checks if the message is sync beacon payload.
func IsSyncBeaconPayload(p *Payload) bool {
	return p.Type() == Type
}

func init() {
	payload.RegisterType(Type, ObjectName, func(data []byte) (payload payload.Payload, err error) {
		payload = &Payload{}
		err = payload.Unmarshal(data)

		return
	})
}
