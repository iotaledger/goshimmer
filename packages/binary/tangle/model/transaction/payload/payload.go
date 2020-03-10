package payload

import (
	"encoding"
)

type Payload interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	Bytes() []byte
	GetType() Type
	String() string
}
