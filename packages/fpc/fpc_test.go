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
		{Opinions{true, true, true, true}, 2, 2, true},
		{Opinions{true, true, true, false}, 2, 2, false},
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
		{Opinions{true, true, true}, true, nil},
		{Opinions{true, true, true, false}, false, nil},
		{Opinions{}, false, errors.New("opinion is empty")},
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
func TestFPC(t *testing.T) {
	type Want struct {
		opinionHistory []Opinion
		finalOpinion   Opinion
	}
	type testInput struct {
		input TxOpinion
		want  Want
	}
	var tests = []testInput{
		{TxOpinion{1, true}, Want{[]Opinion{true, true, true, true, true}, true}},
		{TxOpinion{2, false}, Want{[]Opinion{false, false, false, false, false}, false}},
	}

	for _, test := range tests {
		N := 10
		fpcInstance := make([]*FPC, N)
		getKnownPeers := func() []int {
			nodesList := make([]int, N)
			for i := 0; i < N; i++ {
				nodesList[i] = i
			}
			return nodesList
		}
		queryNode := func(txs []Hash, node int) []Opinion {
			return fpcInstance[node].GetInterimOpinion(txs...)
		}
		for i := 0; i < N; i++ {
			fpcInstance[i] = New(getKnownPeers, queryNode, NewParameters())
			fpcInstance[i].SubmitTxsForVoting(test.input)
		}

		//ticker := time.NewTicker(300 * time.Millisecond)
		finished := make(chan []TxOpinion, N)
		done := 0
		round := 0
		for done < N {
			//<-ticker.C:
			round++
			// start a new round
			for i := 0; i < N; i++ {
				fpcInstance[i].Tick(uint64(round), 0.7)
			}
			// check if done
			for i := 0; i < N; i++ {
				finalizedTxs := <-fpcInstance[i].FinalizedTxsChannel()
				if len(finalizedTxs) > 0 {
					done++
					finished <- finalizedTxs
				}
			}
		}

		nodesFinalOpinion := [][]TxOpinion{}
		for i := 0; i < N; i++ {
			nodesFinalOpinion = append(nodesFinalOpinion, <-finished)
			// check opinion history
			nodeHistory := fpcInstance[i].debug_GetOpinionHistory()
			txHistory, _ := nodeHistory.Load(test.input.TxHash)
			for i, opinionOfRound := range txHistory {
				if len(test.want.opinionHistory) < i+1 || opinionOfRound != test.want.opinionHistory[i] {
					t.Error("Should return", test.want.opinionHistory, "got", txHistory, "with input", test.input)
				}
			}
			// check finalized opinion
			for _, finalOpinion := range nodesFinalOpinion[i] {
				if finalOpinion.Opinion != test.want.finalOpinion {
					t.Error("Should return", test.want.finalOpinion, "got", finalOpinion, "with input", test.input)
				}
			}
		}
	}
}
