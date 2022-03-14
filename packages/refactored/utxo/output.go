package utxo

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
)

type Output interface {
	ID() (id OutputID)

	Bytes() (serializedOutput []byte)

	String() (humanReadableOutput string)

	objectstorage.StorableObject
}
