package discover

import (
	"sync"
	"time"

	log "go.uber.org/zap"
)

const (
	revalidateInterval = 30 * time.Second
)

type store struct {
	p   *protocol
	db  *DB
	log *log.SugaredLogger

	wg      sync.WaitGroup
	closing chan struct{}
}

func newStore(p *protocol, log *log.SugaredLogger) *store {
	s := &store{
		p:       p,
		db:      NewMapDB(log),
		log:     log,
		closing: make(chan struct{}),
	}

	s.wg.Add(1)
	go s.loop()

	return s
}

func (s *store) close() {
	s.log.Debugf("closing")

	close(s.closing)
	s.db.Close()
	s.wg.Wait()
}

func (s *store) loop() {
	defer s.wg.Done()

	var (
		revalidate = time.NewTimer(0) // setting this to 0 will cause a trigger right away

		revalidateDone chan struct{}
	)
	defer revalidate.Stop()

loop:
	for {
		select {
		case <-revalidate.C:
			// if there is no revalidateDone, this means doRevalidate is not running
			if revalidateDone == nil {
				revalidateDone = make(chan struct{})
				go s.doRevalidate(revalidateDone)
			}
		case <-revalidateDone:
			revalidateDone = nil
			revalidate.Reset(revalidateInterval) // revalidate again after the given interval
		case <-s.closing:
			break loop
		}
	}

	// wait for the revalidate to finish
	if revalidateDone != nil {
		<-revalidateDone
	}
}

func (s *store) doRevalidate(done chan<- struct{}) {
	defer func() { done <- struct{}{} }() // always signal, when the function returns

	s.log.Debugf("revalidate")

	id := s.db.GetRandomID()
	if id != "" {
		addr := s.db.Address(id)
		s.p.ping(addr, id)
	}

}
