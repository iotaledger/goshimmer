package utxo

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
)

type Output interface {
	ID() (id OutputID)

	SetID(id OutputID)

	Bytes() (serializedOutput []byte)

	String() (humanReadableOutput string)

	objectstorage.StorableObject
}
