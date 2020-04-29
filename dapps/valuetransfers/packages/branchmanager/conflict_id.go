package branchmanager

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// ConflictID represents an identifier of a Conflict. Since conflicts, are created by multiple transactions spending the
// same Output, the ConflictID is simply an alias for the conflicting OutputID.
type ConflictID = transaction.OutputID

var (
	// ParseConflictID is a wrapper for simplified unmarshaling of Ids from a byte stream using the marshalUtil package.
	ParseConflictID = transaction.ParseOutputID

	// ConflictIDFromBytes unmarshals a ConflictID from a sequence of bytes.
	ConflictIDFromBytes = transaction.OutputIDFromBytes
)

// ConflictIDLength encodes the length of a Conflict identifier - since Conflicts get created by transactions spending
// the same Output, it has the same length as an OutputID.
const ConflictIDLength = transaction.OutputIDLength
