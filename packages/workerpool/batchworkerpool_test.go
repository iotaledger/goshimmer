package workerpool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/curl"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

func Benchmark(b *testing.B) {
	trits := ternary.Trytes("A99999999999999999999999A99999999999999999999999A99999999999999999999999A99999999999999999999999").ToTrits()

	options := DEFAULT_OPTIONS
	options.QueueSize = 5000

	pool := NewBatchWorkerPool(processBatchHashRequests, options)
	pool.Start()

	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		resultChan := pool.Submit(trits)

		wg.Add(1)
		go func() {
			<-resultChan

			wg.Done()
		}()
	}
	wg.Wait()
}

func TestBatchWorkerPool(t *testing.T) {
	pool := NewBatchWorkerPool(processBatchHashRequests, DEFAULT_OPTIONS)
	pool.Start()

	trits := ternary.Trytes("A99999").ToTrits()

	var wg sync.WaitGroup
	for i := 0; i < 10007; i++ {
		resultChan := pool.Submit(trits)

		wg.Add(1)
		go func() {
			<-resultChan

			wg.Done()
		}()
	}
	wg.Wait()
}

func processBatchHashRequests(calls []Call) {
	if len(calls) > 1 {
		// multiplex the requests
		multiplexer := ternary.NewBCTernaryMultiplexer()
		for _, hashRequest := range calls {
			multiplexer.Add(hashRequest.params[0].(ternary.Trits))
		}
		bcTrits, err := multiplexer.Extract()
		if err != nil {
			fmt.Println(err)
		}

		// calculate the hash
		bctCurl := curl.NewBCTCurl(243, 81)
		bctCurl.Reset()
		bctCurl.Absorb(bcTrits)

		// extract the results from the demultiplexer
		demux := ternary.NewBCTernaryDemultiplexer(bctCurl.Squeeze(243))
		for i, hashRequest := range calls {
			hashRequest.resultChan <- demux.Get(i)
			close(hashRequest.resultChan)
		}
	} else {
		var resp = make(ternary.Trits, 243)

		curl := curl.NewCurl(243, 81)
		curl.Absorb(calls[0].params[0].(ternary.Trits), 0, len(calls[0].params[0].(ternary.Trits)))
		curl.Squeeze(resp, 0, 81)

		calls[0].resultChan <- resp
		close(calls[0].resultChan)
	}
}
