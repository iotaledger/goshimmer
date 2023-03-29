package newconflictdag

import (
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
)

type TestID struct {
	utxo.TransactionID
}

func NewTestID(alias string) TestID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	testID := utxo.NewTransactionID(hashedAlias[:])
	testID.RegisterAlias(alias)

	return TestID{testID}
}

func (id TestID) String() string {
	return strings.Replace(id.TransactionID.String(), "TransactionID(", "TestID(", 1)
}
