package peer

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"go.uber.org/zap"
)

// mapDB is a simple implementation of DB using a map.
type mapDB struct {
	mutex    sync.RWMutex
	m        map[string]peerEntry
	key      PrivateKey
	services *service.Record

	log *zap.SugaredLogger

	wg        sync.WaitGroup
	closeOnce sync.Once
	closing   chan struct{}
}

type peerEntry struct {
	data       []byte
	properties map[string]peerPropEntry
}

type peerPropEntry struct {
	lastPing, lastPong int64
}

// NewMemoryDB creates a new DB that uses a GO map.
func NewMemoryDB(log *zap.SugaredLogger) DB {
	db := &mapDB{
		m:        make(map[string]peerEntry),
		services: service.New(),
		log:      log,
		closing:  make(chan struct{}),
	}

	// start the expirer routine
	db.wg.Add(1)
	go db.expirer()

	return db
}

// Close closes the DB.
func (db *mapDB) Close() {
	db.closeOnce.Do(func() {
		db.log.Debugf("closing")
		close(db.closing)
		db.wg.Wait()
	})
}

// LocalPrivateKey returns the private key stored in the database or creates a new one.
func (db *mapDB) LocalPrivateKey() (PrivateKey, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.key == nil {
		key, err := generatePrivateKey()
		db.key = key
		return key, err
	}

	return db.key, nil
}

// LocalServices returns the services stored in the database or creates an empty map.
func (db *mapDB) LocalServices() (service.Service, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	return db.services, nil
}

// UpdateLocalServices updates the services stored in the database.
func (db *mapDB) UpdateLocalServices(services service.Service) error {
	record := services.CreateRecord()

	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.services = record
	return nil
}

// LastPing returns that property for the given peer ID and address.
func (db *mapDB) LastPing(id ID, address string) time.Time {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	peerEntry := db.m[string(id.Bytes())]
	return time.Unix(peerEntry.properties[address].lastPing, 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *mapDB) UpdateLastPing(id ID, address string, t time.Time) error {
	key := string(id.Bytes())

	db.mutex.Lock()
	defer db.mutex.Unlock()

	peerEntry := db.m[key]
	if peerEntry.properties == nil {
		peerEntry.properties = make(map[string]peerPropEntry)
	}
	entry := peerEntry.properties[address]
	entry.lastPing = t.Unix()
	peerEntry.properties[address] = entry
	db.m[key] = peerEntry

	return nil
}

// LastPong returns that property for the given peer ID and address.
func (db *mapDB) LastPong(id ID, address string) time.Time {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	peerEntry := db.m[string(id.Bytes())]
	return time.Unix(peerEntry.properties[address].lastPong, 0)
}

// UpdateLastPong updates that property for the given peer ID and address.
func (db *mapDB) UpdateLastPong(id ID, address string, t time.Time) error {
	key := string(id.Bytes())

	db.mutex.Lock()
	defer db.mutex.Unlock()

	peerEntry := db.m[key]
	if peerEntry.properties == nil {
		peerEntry.properties = make(map[string]peerPropEntry)
	}
	entry := peerEntry.properties[address]
	entry.lastPong = t.Unix()
	peerEntry.properties[address] = entry
	db.m[key] = peerEntry

	return nil
}

// UpdatePeer updates a peer in the database.
func (db *mapDB) UpdatePeer(p *Peer) error {
	data, err := p.Marshal()
	if err != nil {
		return err
	}
	key := string(p.ID().Bytes())

	db.mutex.Lock()
	defer db.mutex.Unlock()

	peerEntry := db.m[key]
	peerEntry.data = data
	db.m[key] = peerEntry

	return nil
}

// Peer retrieves a peer from the database.
func (db *mapDB) Peer(id ID) *Peer {
	db.mutex.RLock()
	peerEntry := db.m[string(id.Bytes())]
	db.mutex.RUnlock()

	if peerEntry.data == nil {
		return nil
	}
	return parsePeer(peerEntry.data)
}

// SeedPeers retrieves random nodes to be used as potential bootstrap peers.
func (db *mapDB) SeedPeers() []*Peer {
	peers := make([]*Peer, 0)
	now := time.Now()

	db.mutex.RLock()
	for id, peerEntry := range db.m {
		p := parsePeer(peerEntry.data)
		if p == nil || id != string(p.ID().Bytes()) {
			continue
		}
		if now.Sub(db.LastPong(p.ID(), p.Address())) > seedExpiration {
			continue
		}

		peers = append(peers, p)
	}
	db.mutex.RUnlock()

	return randomSubset(peers, seedCount)
}

func (db *mapDB) expirer() {
	defer db.wg.Done()

	// the expiring isn't triggert right away, to give the bootstrapping the chance to use older nodes
	tick := time.NewTicker(cleanupInterval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			db.expirePeers()
		case <-db.closing:
			return
		}
	}
}

func (db *mapDB) expirePeers() {
	var (
		threshold = time.Now().Add(-peerExpiration).Unix()
		count     int
	)

	db.mutex.Lock()
	for id, peerEntry := range db.m {
		for address, peerPropEntry := range peerEntry.properties {
			if peerPropEntry.lastPong <= threshold {
				delete(peerEntry.properties, address)
			}
		}
		if len(peerEntry.properties) == 0 {
			delete(db.m, id)
			count++
		}
	}
	db.mutex.Unlock()

	db.log.Debugw("expired peers", "count", count)
}
