package fcob

import (
	"reflect"
	"testing"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// mockedVoter is an empty struct used to satisfy
// the OpinionVoter interface
type mockedVoter struct{}

func (mockedVoter) SubmitTxsForVoting(txs ...fpc.TxOpinion) {}
func (mockedVoter) CancelTxsFromVoting(txs ...fpc.ID)       {}

// mockedUpdater defines the mocked storage
// to store the txs opinions
type mockedOpinioner struct {
	storage map[ternary.Trinary]Opinion
}

// implementation of a mocked version of the Opinioner interface
func (m mockedOpinioner) GetOpinion(transactionHash ternary.Trinary) (opinion Opinion, err errors.IdentifiableError) {
	return m.storage[transactionHash], nil
}

func (m mockedOpinioner) SetOpinion(transactionHash ternary.Trinary, opinion Opinion) (err errors.IdentifiableError) {
	m.storage[transactionHash] = opinion
	return nil
}

func (m mockedOpinioner) Decide(txHash ternary.Trinary) (opinion Opinion, conflictSet map[ternary.Trinary]bool) {
	conflictSet = make(map[ternary.Trinary]bool)
	return Opinion{fpc.Dislike, false}, conflictSet
}

// TestRunProtocol tests the FCoB protocol
func TestRunProtocol(t *testing.T) {
	type testInput struct {
		tx       ternary.Trinary
		expected map[ternary.Trinary]Opinion
	}
	var tests = []testInput{
		{
			tx: ternary.Trinary("1"),
			// we use the dummyConflictCheck, so it should return false
			expected: map[ternary.Trinary]Opinion{"1": Opinion{fpc.Dislike, false}},
		},
	}

	testVoter := mockedVoter{}
	testUpdater := mockedOpinioner{
		storage: make(map[ternary.Trinary]Opinion),
	}
	runProtocol := makeRunProtocol(testVoter, testUpdater)

	for _, test := range tests {
		runProtocol(test.tx)
		if !reflect.DeepEqual(testUpdater.storage, test.expected) {
			t.Error("Should return", test.expected, "got", testUpdater.storage, "with input", test)
		}
	}
}
