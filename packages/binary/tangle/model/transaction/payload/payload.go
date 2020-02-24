package payload

import (
	"encoding"
)

type Payload interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	GetType() Type
}
