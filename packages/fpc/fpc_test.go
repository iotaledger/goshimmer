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

func TestFPC(t *testing.T) {
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
		fpcInstance[i].VoteOnTxs(TxOpinion{1, true})
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
			finalizedTxs := <-fpcInstance[i].FinalizedTxs
			//t.Log("Node", i+1, fpcInstance[i].Debug_GetOpinionHistory())
			if len(finalizedTxs) > 0 {
				done++
				finished <- finalizedTxs
			}
		}
	}

	nodesFinalOpinion := [][]TxOpinion{}
	for i := 0; i < N; i++ {
		nodesFinalOpinion = append(nodesFinalOpinion, <-finished)
	}
	//fmt.Println("Safety:", safety(nodesFinalOpinion...))
	t.Log("All done.", nodesFinalOpinion)
}
