package manaverse

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/priorityqueue"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Scheduler struct {
	queue         *priorityqueue.PriorityQueue[*Bucket]
	bucketsByMana map[uint64]*Bucket
	iterations    int
	sync.RWMutex
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		queue: priorityqueue.New[*Bucket](),
	}
}

func (s *Scheduler) Push(block *tangle.Message) {
	s.Lock()
	defer s.Unlock()

	s.bucket(block.BurnedMana()).Push(block)
}

func (s *Scheduler) Start() {
	go func() {
		tickerRate := 1 * time.Millisecond
		tickerPrecision := 16 * time.Millisecond

		start := time.Now()
		for true {
			s.iterations++

			if tickerOffset := start.Add(time.Duration(s.iterations) * tickerRate).Sub(time.Now()); tickerOffset > tickerPrecision {
				time.Sleep(tickerOffset)
			}
		}
	}()
}

func (s *Scheduler) bucket(mana uint64) (bucket *Bucket) {
	bucket, exists := s.bucketsByMana[mana]
	if exists {
		return bucket
	}

	bucket = NewManaBucket(mana)
	s.bucketsByMana[mana] = bucket

	return bucket
}

type PrecisionTicker struct {
	tickerFunc   func()
	iterations   int
	shutdownWG   sync.WaitGroup
	shutdownChan chan types.Empty
}

func NewPrecisionTicker(tickerFunc func()) (precisionTicker *PrecisionTicker) {
	precisionTicker = &PrecisionTicker{
		tickerFunc:   tickerFunc,
		shutdownChan: make(chan types.Empty, 1),
	}

	go precisionTicker.run()

	return precisionTicker
}

func (p *PrecisionTicker) Shutdown() {
	close(p.shutdownChan)
}

func (p *PrecisionTicker) WaitForShutdown() {
	<-p.shutdownChan
}

func (p *PrecisionTicker) WaitForGracefulShutdown() {
	p.shutdownWG.Wait()
}

func (p *PrecisionTicker) run() {
	p.shutdownWG.Add(1)
	defer p.shutdownWG.Done()

	tickerRate := 1 * time.Millisecond
	tickerPrecision := 16 * time.Millisecond

	start := time.Now()
	for true {
		select {
		case <-p.shutdownChan:
		default:
			p.iterations++

			p.tickerFunc()

			if tickerOffset := start.Add(time.Duration(p.iterations) * tickerRate).Sub(time.Now()); tickerOffset > tickerPrecision {
				time.Sleep(tickerOffset)
			}
		}

	}
}
