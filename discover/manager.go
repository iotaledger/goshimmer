package discover

import (
	"math/rand"
	"sync"
	"time"

	"github.com/wollac/autopeering/peer"
	"go.uber.org/zap"
)

const (
	reverifyInterval = 10 * time.Second
	reverifyTries    = 2

	maxKnow         = 100
	maxReplacements = 10

	bootCount  = 30
	bootMaxAge = 5 * 24 * time.Hour
)

type network interface {
	Local() *peer.Local

	ping(*peer.Peer) error
	requestPeers(*peer.Peer) <-chan error
}

type manager struct {
	net  network
	boot []*peer.Peer
	log  *zap.SugaredLogger

	db           *DB
	known        []*peer.Peer
	replacements []*peer.Peer
	mutex        sync.Mutex

	wg      sync.WaitGroup
	closing chan struct{}
}

func newManager(net network, boot []*peer.Peer, log *zap.SugaredLogger) *manager {
	m := &manager{
		net:          net,
		boot:         boot,
		db:           NewMapDB(log.Named("db")),
		known:        make([]*peer.Peer, 0, maxKnow),
		replacements: make([]*peer.Peer, 0, maxReplacements),
		log:          log,
		closing:      make(chan struct{}),
	}
	m.loadInitialPeers()

	m.wg.Add(1)
	go m.loop()

	return m
}

func (m *manager) close() {
	m.log.Debugf("closing")

	close(m.closing)
	m.db.Close()
	m.wg.Wait()
}

func (m *manager) loop() {
	defer m.wg.Done()

	var (
		reverify = time.NewTimer(0) // setting this to 0 will cause a trigger right away

		reverifyDone chan struct{}
	)
	defer reverify.Stop()

loop:
	for {
		select {
		case <-reverify.C:
			// if there is no reverifyDone, this means doReverify is not running
			if reverifyDone == nil {
				reverifyDone = make(chan struct{})
				go m.doReverify(reverifyDone)
			}
		case <-reverifyDone:
			reverifyDone = nil
			reverify.Reset(reverifyInterval) // reverify again after the given interval
		case <-m.closing:
			break loop
		}
	}

	// wait for the reverify to finish
	if reverifyDone != nil {
		<-reverifyDone
	}
}

// doReverify pings the oldest known peer.
func (m *manager) doReverify(done chan<- struct{}) {
	defer func() { done <- struct{}{} }() // always signal, when the function returns

	last := m.peerToReverify()
	if last == nil {
		return // nothing can be reverify
	}

	var err error
	for i := 0; i < reverifyTries && err != nil; i++ {
		err = m.net.ping(last)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err != nil {
		if len(m.replacements) == 0 {
			m.known = m.known[:len(m.known)-1] // pop back
		} else {
			var r *peer.Peer
			m.replacements, r = deletePeer(m.replacements, rand.Intn(len(m.replacements)))
			m.known[len(m.known)-1] = r
		}

		m.log.Debugw("removed dead node",
			"id", last.Identity,
			"addr", last.Address,
			"err", err,
		)
	} else {
		m.bumpNode(last)

		// trigger a query
		// TODO: this should be independent of the revalidation
		m.net.requestPeers(last)

		m.log.Debugw("reverified node",
			"id", last.Identity,
		)
	}
}

// peerToReverify returns the oldest peer, or nil if empty.
func (m *manager) peerToReverify() *peer.Peer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.known) > 0 {
		// the last peer is the oldest
		return m.known[len(m.known)-1]
	}
	return nil
}

func (m *manager) bumpNode(peer *peer.Peer) bool {
	id := peer.Identity

	for i, p := range m.known {
		if p.Identity.Equal(id) {
			// update and move it to the front
			copy(m.known[1:], m.known[:i])
			m.known[0] = peer
			return true
		}
	}
	return false
}

// pushPeer is a helper function that adds a new peer to the front of the list.
func pushPeer(list []*peer.Peer, p *peer.Peer, max int) []*peer.Peer {
	if len(list) < max {
		list = append(list, nil)
	}
	copy(list[1:], list)
	list[0] = p

	return list
}

// containsPeer is a helper that returns true if the peer is contained in the list.
func containsPeer(list []*peer.Peer, p *peer.Peer) bool {
	id := p.Identity

	for _, p := range list {
		if p.Identity.Equal(id) {
			return true
		}
	}
	return false
}

// deletePeer is a helper that deletes the peer with the given index from the list.
func deletePeer(list []*peer.Peer, i int) ([]*peer.Peer, *peer.Peer) {
	p := list[i]

	copy(list[i:], list[i+1:])
	list[len(list)-1] = nil

	return list[:len(list)-1], p
}

func (m *manager) addReplacement(p *peer.Peer) {
	if containsPeer(m.replacements, p) {
		return // already in the list
	}
	m.replacements = pushPeer(m.replacements, p, maxReplacements)
}

func (m *manager) loadInitialPeers() {
	peers := m.db.RandomPeers(bootCount, bootMaxAge)
	peers = append(peers, m.boot...)
	for _, peer := range peers {
		m.addDiscoveredPeer(peer)
	}
}

// addDiscoveredPeer adds a newly discovered peer that has never been verified or pinged yet.
func (m *manager) addDiscoveredPeer(p *peer.Peer) {
	// never add the local peer
	if m.net.Local().Private.Equal(p.Identity) {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if containsPeer(m.known, p) {
		return
	}

	m.log.Debugw("addDiscoveredPeer",
		"id", p.Identity,
		"addr", p.Address,
	)

	if len(m.known) < maxKnow {
		m.known = append(m.known, p)
	} else {
		m.addReplacement(p)
	}
}

// addVerifiedPeer adds a new peer that has just been successfully pinged.
func (m *manager) addVerifiedPeer(p *peer.Peer) {
	// never add the local peer
	if m.net.Local().Private.Equal(p.Identity) {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// if already in the list, move it to the front
	if m.bumpNode(p) {
		return
	}

	m.log.Debugw("addVerifiedPeer",
		"id", p.Identity,
		"addr", p.Address,
	)
	// new nodes are added to the front
	m.known = pushPeer(m.known, p, maxKnow)
}

func (m *manager) getRandomPeers(n int) []*peer.Peer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if n > len(m.known) {
		n = len(m.known)
	}

	peers := make([]*peer.Peer, 0, n)
	for _, i := range rand.Perm(len(m.known)) {
		peers = append(peers, m.known[i])
	}

	return peers
}
