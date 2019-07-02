package fcob

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// mockedVoter is an empty struct used to satisfy
// the Voter interface
type mockedVoter struct{}

func (mockedVoter) SubmitTxsForVoting(txs ...fpc.TxOpinion) {}

type mockedTangle struct {
	tangle   map[ternary.Trytes]*value_transaction.ValueTransaction
	metadata map[ternary.Trytes]*transactionmetadata.TransactionMetadata
	hashToID map[ternary.Trytes]ternary.Trytes
}

func (m mockedTangle) GetTransaction(transactionHash ternary.Trytes, computeIfAbsent ...func(ternary.Trytes) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError) {
	return m.tangle[m.hashToID[transactionHash]], nil
}

func (m mockedTangle) GetTransactionMetadata(transactionHash ternary.Trytes, computeIfAbsent ...func(ternary.Trytes) *transactionmetadata.TransactionMetadata) (result *transactionmetadata.TransactionMetadata, err errors.IdentifiableError) {
	return m.metadata[m.hashToID[transactionHash]], nil
}

func (m mockedTangle) new(value int64, ID, branch, trunk ternary.Trytes, like, final bool) {
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
	mockedDB.tangle = make(map[ternary.Trytes]*value_transaction.ValueTransaction)
	mockedDB.metadata = make(map[ternary.Trytes]*transactionmetadata.TransactionMetadata)
	mockedDB.hashToID = make(map[ternary.Trytes]ternary.Trytes)

	type testInput struct {
		db            mockedTangle
		tx            ternary.Trytes
		value         int64
		branch        ternary.Trytes
		trunk         ternary.Trytes
		expectedLiked bool
		expectedVoted bool
	}
	var tests = []testInput{
		// test for conflict
		{
			db:            mockedDB,
			tx:            ternary.Trytes("4"),
			value:         10, //currently value%10 triggers a new conflict
			branch:        ternary.Trytes("2"),
			trunk:         ternary.Trytes("2"),
			expectedLiked: DISLIKED,
			expectedVoted: UNVOTED,
		},
		// test for monotonicity
		{
			db:            mockedDB,
			tx:            ternary.Trytes("5"),
			value:         11,
			branch:        ternary.Trytes("2"),
			trunk:         ternary.Trytes("3"),
			expectedLiked: DISLIKED,
			expectedVoted: VOTED,
		},
		// test for no conflict
		{
			db:            mockedDB,
			tx:            ternary.Trytes("6"),
			value:         12,
			branch:        ternary.Trytes("2"),
			trunk:         ternary.Trytes("2"),
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
