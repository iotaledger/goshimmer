package discover

import (
	"container/ring"
	"math/rand"
	"sync"
	"time"
)

// doQuery is the main method of the query strategy.
// It writes the next time this function should be called by the manager to next.
// The current strategy is to always select the latest verified peer and one of
// the peers that returned the most number of peers the last time it was queried.
func (m *manager) doQuery(next chan<- time.Duration) {
	defer func() { next <- queryInterval }()

	ps := m.peersToQuery()
	if len(ps) == 0 {
		return
	}
	m.log.Debugw("querying",
		"#peers", len(ps),
	)

	// request from peers in parallel
	var wg sync.WaitGroup
	wg.Add(len(ps))
	for _, p := range ps {
		go m.requestWorker(p, &wg)
	}
	wg.Wait()
}

func (m *manager) requestWorker(p *mpeer, wg *sync.WaitGroup) {
	defer wg.Done()

	r, err := m.net.discoveryRequest(unwrapPeer(p))
	if err != nil || len(r) == 0 {
		p.lastNewPeers = 0

		m.log.Debugw("query failed",
			"id", p.ID(),
			"addr", p.Address(),
			"err", err,
		)
		return
	}

	var added uint
	for _, rp := range r {
		if m.addDiscoveredPeer(rp) {
			added++
		}
	}
	p.lastNewPeers = added

	m.log.Debugw("queried",
		"id", p.ID(),
		"addr", p.Address(),
		"#added", added,
	)
}

// peersToQuery selects the peers that should be queried.
func (m *manager) peersToQuery() []*mpeer {
	ps := m.getVerifiedPeers()
	if len(ps) == 0 {
		return nil
	}

	latest := ps[0]
	if len(ps) == 1 {
		return []*mpeer{latest}
	}

	// find the 3 heaviest peers
	r := ring.New(3)
	for i, p := range ps {
		if i == 0 {
			continue // the latest peer is already included
		}
		if r.Value == nil {
			r.Value = p
		} else if p.lastNewPeers >= r.Value.(*mpeer).lastNewPeers {
			r = r.Next()
			r.Value = p
		}
	}

	// select a random peer from the heaviest ones
	r.Move(rand.Intn(r.Len()))

	return []*mpeer{latest, r.Value.(*mpeer)}
}
