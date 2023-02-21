package utxo

import (
	"github.com/iotaledger/hive.go/objectstorage/generic"
)

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction is the type that is used to describe instructions how to modify the ledger state.
type Transaction interface {
	// ID returns the identifier of the Transaction.
	ID() (id TransactionID)

	// SetID sets the identifier of the Transaction.
	//
	// Note: Since determining the identifier of a Transaction is an operation that requires at least a few hashing
	// operations, we allow the ID to be set from the outside.
	//
	// This allows us to potentially skip the ID calculation in cases where the ID is known upfront (e.g. when loading
	// Transactions from the object storage).
	SetID(id TransactionID)

	// Inputs returns the inputs of the Transaction.
	Inputs() (inputs []Input)

	// String returns a human-readable version of the Transaction.
	String() (humanReadable string)

	generic.StorableObject
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Input ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Input is an entity that allows to "address" which Outputs are supposed to be used by a Transaction.
type Input interface {
	// String returns a human-readable version of the Input.
	String() (humanReadable string)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is the container for the data produced by executing a Transaction.
type Output interface {
	// ID returns the identifier of the Output.
	ID() (id OutputID)

	// SetID sets the identifier of the Output.
	SetID(id OutputID)

	// Bytes returns a serialized version of the Output.
	Bytes() (serialized []byte, err error)

	// String returns a human-readable version of the Output.
	String() (humanReadable string)

	generic.StorableObject
}

type OutputFactory func([]byte) (output Output, err error)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Snapshot /////////////////////////////////////////////////////////////////////////////////////////////////////

// Snapshot is a snapshot of the ledger state.
type Snapshot interface {
	Outputs() (outputs []Output)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
