package discover

import (
	"math/rand"
	"sync"
	"time"

	log "go.uber.org/zap"
)

const (
	revalidateInterval = 10 * time.Second

	bucketSize      = 100
	maxReplacements = 10
)

type network interface {
	ping(*Peer) error
}

type store struct {
	net network
	log *log.SugaredLogger

	db           *DB
	known        []*Peer
	replacements []*Peer
	mutex        sync.Mutex

	wg      sync.WaitGroup
	closing chan struct{}
}

func newStore(net network, log *log.SugaredLogger) *store {
	s := &store{
		net:          net,
		db:           NewMapDB(log),
		known:        make([]*Peer, 0, bucketSize),
		replacements: make([]*Peer, 0, maxReplacements),
		log:          log,
		closing:      make(chan struct{}),
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

// doRevalidate pings the oldest known peer.
func (s *store) doRevalidate(done chan<- struct{}) {
	defer func() { done <- struct{}{} }() // always signal, when the function returns

	peer := s.peerToRevalidate()
	if peer == nil {
		return // nothing can be revalidate
	}

	err := s.net.ping(peer)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err != nil {
		if len(s.replacements) == 0 {
			s.known = s.known[:len(s.known)-1] // pop back
		} else {
			var r *Peer
			s.replacements, r = deletePeer(s.replacements, rand.Intn(len(s.replacements)))
			s.known[len(s.known)-1] = r
		}

		s.log.Debug("removed dead node", "id", peer.Identity.StringID, "addr", peer.Address, "err", err)
	} else {
		s.bumpNode(peer)

		s.log.Debug("revalidated node", "id", peer.Identity.StringID)
	}
}

// peerToRevalidate returns the oldest peer, or nil if empty.
func (s *store) peerToRevalidate() *Peer {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.known) > 0 {
		// the last peer is the oldest
		return s.known[len(s.known)-1]
	}
	return nil
}

func (s *store) bumpNode(peer *Peer) bool {
	id := peer.Identity

	for i, p := range s.known {
		if p.Identity.StringID == id.StringID {
			// update and move it to the front
			copy(s.known[1:], s.known[:i])
			s.known[0] = peer
			return true
		}
	}
	return false
}

// pushPeer is a helper function that adds a new peer to the front of the list.
func pushPeer(list []*Peer, p *Peer, max int) []*Peer {
	if len(list) < max {
		list = append(list, nil)
	}
	copy(list[1:], list)
	list[0] = p

	return list
}

// containsPeer is a helper that returns true if the peer is contained in the list.
func containsPeer(list []*Peer, p *Peer) bool {
	id := p.Identity

	for _, p := range list {
		if p.Identity.StringID == id.StringID {
			return true
		}
	}
	return false
}

// deletePeer is a helper that deletes the peer with the given index from the list.
func deletePeer(list []*Peer, i int) ([]*Peer, *Peer) {
	p := list[i]

	copy(list[i:], list[i+1:])
	list[len(list)-1] = nil

	return list[:len(list)-1], p
}

func (s *store) addReplacement(p *Peer) {
	if containsPeer(s.replacements, p) {
		return // already in the list
	}
	s.replacements = pushPeer(s.replacements, p, maxReplacements)
}

// addDiscoveredPeer adds a newly discovered peer that has never been verified or pinged yet.
func (s *store) addDiscoveredPeer(p *Peer) {
	// TODO: ignore self

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if containsPeer(s.known, p) {
		return
	}
	if len(s.known) < bucketSize {
		s.known = append(s.known, p)
	} else {
		s.addReplacement(p)
	}
}

// addVerifiedPeer adds a new peer that has just been successfully pinged.
func (s *store) addVerifiedPeer(peer *Peer) {
	// TODO: ignore self

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// if already in the list, move it to the front
	if s.bumpNode(peer) {
		return
	}
	// new nodes are added to the front
	s.known = pushPeer(s.known, peer, bucketSize)
}
