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
		want     Opinion
		err      error
	}
	var tests = []testInput{
		{Opinions{Like, Like, Like}, Like, nil},
		{Opinions{Like, Like, Like, Dislike}, Dislike, nil},
		{Opinions{}, Undefined, errors.New("opinion is empty")},
	}

	for _, test := range tests {
		result, err := getLastOpinion(test.opinions)
		if result != test.want || !reflect.DeepEqual(err, test.err) {
			t.Error("Should return", test.want, test.err, "got", result, err, "with input", test)
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
		{TxOpinion{"1", Like}, Expected{[]Opinion{Like, Like, Like, Like, Like}, Like}},
		{TxOpinion{"2", Dislike}, Expected{[]Opinion{Dislike, Dislike, Dislike, Dislike, Dislike}, Dislike}},
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
		// check opinion history
		nodeHistory := fpcInstance.state.opinionHistory
		txHistory, _ := nodeHistory.Load(test.input.TxHash)
		if len(txHistory) != len(test.expected.opinionHistory) {
			t.Error("Should return", test.expected.opinionHistory, "got", txHistory, "with input", test.input)
		}
		for i, opinionOfRound := range txHistory {
			if len(test.expected.opinionHistory) < i+1 || opinionOfRound != test.expected.opinionHistory[i] {
				t.Error("Should return", test.expected.opinionHistory, "got", txHistory, "with input", test.input)
			}
		}
		// check finalized opinion
		for _, finalOpinion := range finalOpinions {
			if finalOpinion.Opinion != test.expected.finalOpinion {
				t.Error("Should return", test.expected.finalOpinion, "got", finalOpinion, "with input", test.input)
			}
		}
	}
}
