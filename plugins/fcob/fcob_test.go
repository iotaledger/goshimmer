package fcob

import (
	"reflect"
	"strconv"
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

// mockedOpinioner defines the mocked storage to store the txs opinions,
// and fulfills the Opinioner interface
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

func (m mockedOpinioner) Decide(txHash ternary.Trinary) (opinion Opinion, conflictSet map[ternary.Trinary]bool, err errors.IdentifiableError) {
	conflictSet = make(map[ternary.Trinary]bool)
	txHashInt, _ := strconv.Atoi(string(txHash))
	switch txHashInt % 2 {
	case 0:
		conflictSet[txHash] = true
		return Opinion{fpc.Dislike, false}, conflictSet, nil
	default:
		return Opinion{fpc.Like, false}, conflictSet, nil
	}
}

// TestRunProtocol tests the FCoB protocol
func TestRunProtocol(t *testing.T) {
	type testInput struct {
		tx       ternary.Trinary
		expected map[ternary.Trinary]Opinion
	}
	var tests = []testInput{
		{
			tx: ternary.Trinary("2"),
			// tx is even so it must save dislike
			expected: map[ternary.Trinary]Opinion{"2": Opinion{fpc.Dislike, false}},
		},
		{
			tx: ternary.Trinary("1"),
			// tx is odd so it must save like
			expected: map[ternary.Trinary]Opinion{"1": Opinion{fpc.Like, false}},
		},
	}

	for _, test := range tests {
		testVoter := mockedVoter{}
		testUpdater := mockedOpinioner{
			storage: make(map[ternary.Trinary]Opinion),
		}
		runProtocol := makeRunProtocol(testVoter, testUpdater)

		runProtocol(test.tx)

		if !reflect.DeepEqual(testUpdater.storage, test.expected) {
			t.Error("Should return", test.expected, "got", testUpdater.storage, "with input", test)
		}
	}
}
