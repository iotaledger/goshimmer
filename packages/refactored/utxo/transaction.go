package utxo

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
)

type Transaction interface {
	ID() TransactionID

	Inputs() []Input

	Bytes() []byte

	objectstorage.StorableObject
}
