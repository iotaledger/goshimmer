package curl

import (
	"fmt"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/batchworkerpool"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/iota.go/trinary"
)

type BatchHasher struct {
	hashLength int
	rounds     int
	workerPool *batchworkerpool.BatchWorkerPool
}

func NewBatchHasher(hashLength int, rounds int) (result *BatchHasher) {
	result = &BatchHasher{
		hashLength: hashLength,
		rounds:     rounds,
	}

	result.workerPool = batchworkerpool.New(result.processHashes, batchworkerpool.BatchSize(strconv.IntSize))
	result.workerPool.Start()

	return
}

func (this *BatchHasher) Hash(trits trinary.Trits) trinary.Trits {
	return (<-this.workerPool.Submit(trits)).(trinary.Trits)
}

func (this *BatchHasher) processHashes(tasks []batchworkerpool.Task) {
	if len(tasks) > 1 {
		// multiplex the requests
		multiplexer := ternary.NewBCTernaryMultiplexer()
		for _, hashRequest := range tasks {
			multiplexer.Add(hashRequest.Param(0).(trinary.Trits))
		}
		bcTrits, err := multiplexer.Extract()
		if err != nil {
			fmt.Println(err)
		}

		// calculate the hash
		bctCurl := NewBCTCurl(this.hashLength, this.rounds)
		bctCurl.Reset()
		bctCurl.Absorb(bcTrits)

		// extract the results from the demultiplexer
		demux := ternary.NewBCTernaryDemultiplexer(bctCurl.Squeeze(243))
		for i, task := range tasks {
			task.Return(demux.Get(i))
		}
	} else {
		var resp = make(trinary.Trits, this.hashLength)

		trits := tasks[0].Param(0).(trinary.Trits)

		curl := NewCurl(this.hashLength, this.rounds)
		curl.Absorb(trits, 0, len(trits))
		curl.Squeeze(resp, 0, this.hashLength)

		tasks[0].Return(resp)
	}
}
