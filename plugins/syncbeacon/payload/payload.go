package payload

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// ObjectName defines the name of the syncbeacon object.
	ObjectName = "syncbeacon"
)

// Type is the type of the syncbeacon payload.
var Type = payload.NewType(200, ObjectName, func(data []byte) (payload payload.Payload, err error) {
	payload, _, err = FromBytes(data)

	return
})

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
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read data
	result = &Payload{}
	_, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse payload size of syncbeacon payload: %w", err)
		return
	}
	result.payloadType, err = payload.TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = fmt.Errorf("failed to parse payload type of syncbeacon payload: %w", err)
		return
	}
	result.sentTime, err = marshalUtil.ReadInt64()
	if err != nil {
		err = fmt.Errorf("failed to parse sent time of syncbeacon payload: %w", err)
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
	objectLength := marshalutil.Int64Size

	// marshal the p specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(objectLength))
	marshalUtil.WriteBytes(Type.Bytes())
	marshalUtil.WriteInt64(p.sentTime)

	// return result
	return marshalUtil.Bytes()
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
