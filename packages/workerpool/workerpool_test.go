package workerpool

import (
	"sync"
	"testing"
)

func Benchmark(b *testing.B) {
	pool := New(func(task Task) {
		task.Return(task.Param(0))
	}, WorkerCount(10), QueueSize(2000))
	pool.Start()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)

		go func() {
			<-pool.Submit(i)

			wg.Done()
		}()
	}

	wg.Wait()
}
