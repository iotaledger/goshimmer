package fcob

import (
	"reflect"
	"testing"

	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// mockedVoter is an empty struct used to satisfy
// the OpinionVoter interface
type mockedVoter struct{}

func (mockedVoter) SubmitTxsForVoting(txs ...fpc.TxOpinion) {}

// mockedMetadata defines the fields updated
// by an opinion update operation
type mockedMetadata struct {
	like  bool
	final bool
}

// mockedUpdater defines the mocked storage
// to store the txs opinions
type mockedUpdater struct {
	storage map[ternary.Trinary]mockedMetadata
}

// UpdateOpinion implements a mocked version of the OpinionUpdater interface
func (m mockedUpdater) UpdateOpinion(txHash ternary.Trinary, opinion fpc.Opinion, final bool) {
	like := true
	if opinion == fpc.Dislike {
		like = false
	}
	m.storage[txHash] = mockedMetadata{like, final}
}

// TestRunProtocol tests the FCoB protocol
func TestRunProtocol(t *testing.T) {
	type testInput struct {
		tx       ternary.Trinary
		expected map[ternary.Trinary]mockedMetadata
	}
	var tests = []testInput{
		{
			tx: ternary.Trinary("1"),
			// we use the dummyConflictCheck, so it should return false
			expected: map[ternary.Trinary]mockedMetadata{"1": mockedMetadata{false, false}},
		},
	}

	testVoter := mockedVoter{}
	testUpdater := mockedUpdater{
		storage: make(map[ternary.Trinary]mockedMetadata),
	}
	runProtocol := makeRunProtocol(testVoter, testUpdater)

	for _, test := range tests {
		runProtocol(test.tx)
		if !reflect.DeepEqual(testUpdater.storage, test.expected) {
			t.Error("Should return", test.expected, "got", testUpdater.storage, "with input", test)
		}
	}
}
