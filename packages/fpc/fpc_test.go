package fpc

import (
	"reflect"
	"testing"
)

func TestIsFinal(t *testing.T) {
	type testInput struct {
		opinions []Opinion
		m        int
		l        int
		want     bool
	}
	var tests = []testInput{
		{[]Opinion{Like, Like, Like, Like}, 2, 2, true},
		{[]Opinion{Like, Like, Like, Dislike}, 2, 2, false},
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
		opinions []Opinion
		expected Opinion
		ok       bool
	}
	var tests = []testInput{
		{[]Opinion{Like, Like, Like}, Like, true},
		{[]Opinion{Like, Like, Like, Dislike}, Dislike, true},
		{[]Opinion{}, Dislike, false},
	}

	for _, test := range tests {
		result, ok := getLastOpinion(test.opinions)
		if result != test.expected || !reflect.DeepEqual(ok, test.ok) {
			t.Error("Should return", test.expected, test.ok, "got", result, ok, "with input", test)
		}
	}
}

func TestGetInterimOpinion(t *testing.T) {
	type testInput struct {
		opinionMap       map[ID][]Opinion
		txs              []ID
		expectedOpinions []Opinion
	}
	var tests = []testInput{
		{map[ID][]Opinion{"1": []Opinion{Like, Like, Like}}, []ID{"1"}, []Opinion{Like}},
		{map[ID][]Opinion{"1": []Opinion{Like, Like, Like}}, []ID{"2"}, []Opinion{Dislike}},
		{map[ID][]Opinion{"1": []Opinion{Like}, "2": []Opinion{Dislike}}, []ID{"1", "2", "3"}, []Opinion{Like, Dislike, Dislike}},
		{map[ID][]Opinion{}, []ID{"1"}, []Opinion{Dislike}},
	}
	for _, test := range tests {
		dummyFpc := &Instance{
			state: newContext(),
		}
		// set opinion history
		dummyFpc.state.opinionHistory.internal = test.opinionMap
		opinions := make([]Opinion, len(test.txs))
		for i, tx := range test.txs {
			opinions[i], _ = dummyFpc.GetInterimOpinion(tx)
		}
		if !reflect.DeepEqual(opinions, test.expectedOpinions) {
			t.Error("Should return", test.expectedOpinions, "got", opinions, "with input", test)
		}
	}
}

// TestFPC checks that given to each node a tx with all 1s or all 0s,
// its opinion history and finalized opinions are all 1s or all 0s wrt the given input
func TestVoteIfAllAgrees(t *testing.T) {
	type Expected struct {
		opinionHistory []Opinion
		finalOpinion   Opinion
	}
	type testInput struct {
		input    TxOpinion
		expected Expected
	}
	var tests = []testInput{
		{TxOpinion{"1", true}, Expected{[]Opinion{Like, Like, Like, Like, Like}, true}},
		{TxOpinion{"2", false}, Expected{[]Opinion{Dislike, Dislike, Dislike, Dislike, Dislike}, false}},
	}

	for _, test := range tests {
		getKnownPeers := func() []string {
			return []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
		}
		queryNode := func(txs []ID, node string) []Opinion {
			reply := []Opinion{}
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
