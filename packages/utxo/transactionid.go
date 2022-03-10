package utxo

import (
	"github.com/mr-tron/base58"
)

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [32]byte

// Bytes returns a marshaled version of the TransactionID.
func (t TransactionID) Bytes() (serializedTransaction []byte) {
	return t[:]
}

// String creates a human-readable version of the TransactionID.
func (t TransactionID) String() (humanReadableTransactionID string) {
	return "TransactionID(" + base58.Encode(t[:]) + ")"
}
