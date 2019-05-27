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

// TestFPC checks that given to each node 2 txs with all 1s and all 0s respectively,
// both opinion history and finalized opinions are all 1s or all 0s wrt the given input
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
		fpcInstance[i].VoteOnTxs(TxOpinion{1, true}, TxOpinion{2, false})
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
		// check opinion history
		//nodeHistory := fpcInstance[i].Debug_GetOpinionHistory()
		// for txHash := range []Hash{1,2} {
		// 	for _, v := range nodeHistory {
		// 		if index == 0 {}
		// 	}
		// }
		//t.Log("Node", i+1, fpcInstance[i].Debug_GetOpinionHistory())
	}

	//fmt.Println("Safety:", safety(nodesFinalOpinion...))
	//t.Log("All done.")
}

// func Benchmark(b *testing.B) {
//     for i := 0; i < b.N; i++ {
// 		// perform the operation we're analyzing
// 		TestFPC(nil)
//     }
// }
