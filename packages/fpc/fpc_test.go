package fpc

import (
	"errors"
	"reflect"
	"testing"
)

func TestIsFinal(t *testing.T) {
	type testInput struct {
		opinions Opinions
		m        int
		l        int
		want     bool
	}
	var tests = []testInput{
		{Opinions{Like, Like, Like, Like}, 2, 2, true},
		{Opinions{Like, Like, Like, Dislike}, 2, 2, false},
		{nil, 2, 2, false},
	}

	for _, test := range tests {
		result := isFinal(test.opinions, test.m, test.l)
		if result != test.want {
			t.Error("Should return", test.want, "got", result, "with input", test)
		}
	}
}

func TestGetLastOpinion(t *testing.T) {
	type testInput struct {
		opinions Opinions
		expected bool
		err      error
	}
	var tests = []testInput{
		{Opinions{Like, Like, Like}, Like, nil},
		{Opinions{Like, Like, Like, Dislike}, Dislike, nil},
		{Opinions{}, Dislike, errors.New("opinion is empty")},
	}

	for _, test := range tests {
		result, err := getLastOpinion(test.opinions)
		if result != test.expected || !reflect.DeepEqual(err, test.err) {
			t.Error("Should return", test.expected, test.err, "got", result, err, "with input", test)
		}
	}
}

func TestGetInterimOpinion(t *testing.T) {
	type testInput struct {
		opinionMap map[ID]Opinions
		txs        []ID
		expected   Opinions
	}
	var tests = []testInput{
		{map[ID]Opinions{"1": Opinions{Like, Like, Like}}, []ID{"1"}, Opinions{Like}},
		{map[ID]Opinions{"1": Opinions{Like, Like, Like}}, []ID{"2"}, Opinions{Dislike}},
		{map[ID]Opinions{"1": Opinions{Like}, "2": Opinions{Dislike}}, []ID{"1", "2", "3"}, Opinions{Like, Dislike, Dislike}},
		{map[ID]Opinions{}, []ID{"1"}, Opinions{Dislike}},
	}
	for _, test := range tests {
		dummyFpc := &Instance{
			state: newContext(),
		}
		// set opinion history
		dummyFpc.state.opinionHistory.internal = test.opinionMap

		result := dummyFpc.GetInterimOpinion(test.txs...)
		if !reflect.DeepEqual(result, test.expected) {
			t.Error("Should return", test.expected, "got", result, "with input", test)
		}
	}
}

// TestFPC checks that given to each node a tx with all 1s or all 0s,
// its opinion history and finalized opinions are all 1s or all 0s wrt the given input
func TestVoteIfAllAgrees(t *testing.T) {
	type Expected struct {
		opinionHistory Opinions
		finalOpinion   bool
	}
	type testInput struct {
		input    TxOpinion
		expected Expected
	}
	var tests = []testInput{
		{TxOpinion{"1", true}, Expected{Opinions{Like, Like, Like, Like, Like}, true}},
		{TxOpinion{"2", false}, Expected{Opinions{Dislike, Dislike, Dislike, Dislike, Dislike}, false}},
	}

	for _, test := range tests {
		getKnownPeers := func() []string {
			return []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
		}
		queryNode := func(txs []ID, node string) Opinions {
			reply := Opinions{}
			for _, tx := range txs {
				if tx == test.input.TxHash {
					reply = append(reply, test.input.Opinion)
				}
			}
			return reply
		}

		fpcInstance := New(getKnownPeers, queryNode, NewParameters())
		Fpc(fpcInstance).SubmitTxsForVoting(test.input)

		finalOpinions := []TxOpinion{}
		round := uint64(0)
		for len(finalOpinions) == 0 {
			// start a new round
			round++
			fpcInstance.Tick(round, 0.7)
			// check if got a finalized tx:
			// FinalizedTxsChannel() returns, after the current round is done,
			// a list of FinalizedTxs, which can be empty
			finalOpinions = <-Fpc(fpcInstance).FinalizedTxsChannel()
		}
		// check finalized opinion
		for _, finalOpinion := range finalOpinions {
			if finalOpinion.Opinion != test.expected.finalOpinion {
				t.Error("Should return", test.expected.finalOpinion, "got", finalOpinion, "with input", test.input)
			}
		}
	}
}
