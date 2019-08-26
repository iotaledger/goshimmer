package discover

import (
	"sync"
	"time"

	log "go.uber.org/zap"
)

const (
	revalidateInterval = 30 * time.Second

	bucketSize = 100
)

type network interface {
	ping(*Peer) error
}

type store struct {
	net    network
	db     *DB
	bucket []*Peer
	log    *log.SugaredLogger

	wg      sync.WaitGroup
	closing chan struct{}
}

func newStore(net network, log *log.SugaredLogger) *store {
	s := &store{
		net:     net,
		db:      NewMapDB(log),
		bucket:  make([]*Peer, 0, bucketSize),
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

	// nothing can be revalidated
	if len(s.bucket) == 0 {
		return
	}

	var peer *Peer

	// pop front
	peer, s.bucket = s.bucket[0], s.bucket[1:]

	err := s.net.ping(peer)
	if err != nil {
		s.bucket = append(s.bucket, peer)
		s.log.Debug("removed dead node", "id", peer.Identity.StringID, "addr", peer.Address, "err", err)
	} else {
		s.log.Debug("revalidated node", "id", peer.Identity.StringID)
	}
}

func (s *store) bumpNode(peer *Peer) bool {
	id := peer.Identity

	for i, p := range s.bucket {
		if p.Identity.StringID == id.StringID {
			// update and move it to the back
			copy(s.bucket[i:], s.bucket[i+1:])
			s.bucket[len(s.bucket)-1] = peer
			return true
		}
	}
	return false
}

func (s *store) addNode(peer *Peer) {
	// TODO: ignore self

	if s.bumpNode(peer) {
		return
	}
	if len(s.bucket) < bucketSize {
		s.bucket = append(s.bucket, peer)
	} else {
		s.bucket[bucketSize-1] = peer
	}
}
