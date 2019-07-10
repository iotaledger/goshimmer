package fcob

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/iota.go/trinary"
)

// mockedVoter is an empty struct used to satisfy
// the Voter interface
type mockedVoter struct{}

func (mockedVoter) SubmitTxsForVoting(txs ...fpc.TxOpinion) {}

type mockedTangle struct {
	tangle   map[trinary.Trytes]*value_transaction.ValueTransaction
	metadata map[trinary.Trytes]*transactionmetadata.TransactionMetadata
	hashToID map[trinary.Trytes]trinary.Trytes
}

func (m mockedTangle) GetTransaction(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError) {
	return m.tangle[m.hashToID[transactionHash]], nil
}

func (m mockedTangle) GetTransactionMetadata(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *transactionmetadata.TransactionMetadata) (result *transactionmetadata.TransactionMetadata, err errors.IdentifiableError) {
	return m.metadata[m.hashToID[transactionHash]], nil
}

func (m mockedTangle) new(value int64, ID, branch, trunk trinary.Trytes, like, final bool) {
	tx := value_transaction.New()
	tx.SetValue(value)
	m.tangle[ID] = value_transaction.FromMetaTransaction(tx.MetaTransaction)
	if ID != "1" {
		m.tangle[ID].SetBranchTransactionHash(m.tangle[branch].GetHash())
		m.tangle[ID].SetTrunkTransactionHash(m.tangle[trunk].GetHash())
	}
	m.hashToID[tx.GetHash()] = ID
	m.metadata[ID] = transactionmetadata.New(tx.GetHash())
	m.metadata[ID].SetLiked(like)
	m.metadata[ID].SetFinalized(final)
}

func (m mockedTangle) createThreeTxsTangle() {
	m.new(1, "1", "1", "1", LIKED, VOTED)
	m.new(2, "2", "1", "1", LIKED, UNVOTED)
	m.new(3, "3", "1", "1", DISLIKED, VOTED)
}

// TestRunProtocol tests the FCoB protocol
func TestRunProtocol(t *testing.T) {
	testVoter := mockedVoter{}
	// initialize tangle
	mockedDB := mockedTangle{}
	mockedDB.tangle = make(map[trinary.Trytes]*value_transaction.ValueTransaction)
	mockedDB.metadata = make(map[trinary.Trytes]*transactionmetadata.TransactionMetadata)
	mockedDB.hashToID = make(map[trinary.Trytes]trinary.Trytes)

	type testInput struct {
		db            mockedTangle
		tx            trinary.Trytes
		value         int64
		branch        trinary.Trytes
		trunk         trinary.Trytes
		expectedLiked bool
		expectedVoted bool
	}
	var tests = []testInput{
		// test for conflict
		{
			db:            mockedDB,
			tx:            trinary.Trytes("4"),
			value:         10, //currently value%10 triggers a new conflict
			branch:        trinary.Trytes("2"),
			trunk:         trinary.Trytes("2"),
			expectedLiked: DISLIKED,
			expectedVoted: UNVOTED,
		},
		// test for monotonicity
		{
			db:            mockedDB,
			tx:            trinary.Trytes("5"),
			value:         11,
			branch:        trinary.Trytes("2"),
			trunk:         trinary.Trytes("3"),
			expectedLiked: DISLIKED,
			expectedVoted: VOTED,
		},
		// test for no conflict
		{
			db:            mockedDB,
			tx:            trinary.Trytes("6"),
			value:         12,
			branch:        trinary.Trytes("2"),
			trunk:         trinary.Trytes("2"),
			expectedLiked: LIKED,
			expectedVoted: UNVOTED,
		},
	}

	for _, test := range tests {
		test.db.createThreeTxsTangle()
		runProtocol := makeRunProtocol(nil, test.db, testVoter)
		test.db.new(test.value, test.tx, test.branch, test.trunk, false, false)

		runProtocol(test.db.tangle[test.tx].GetHash())

		if test.db.metadata[test.tx].GetLiked() != test.expectedLiked {
			t.Error("Liked status - Should return", test.expectedLiked, "got", test.db.metadata[test.tx].GetLiked(), "with input", test)
		}
		if test.db.metadata[test.tx].GetFinalized() != test.expectedVoted {
			t.Error("Voted status - Should return", test.expectedVoted, "got", test.db.metadata[test.tx].GetFinalized(), "with input", test)
		}
	}
}
