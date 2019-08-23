package discover

import (
	"math/rand"
	"sync"
	"time"

	log "go.uber.org/zap"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour
)

type DB struct {
	m   map[string]*dbEntry
	mu  sync.RWMutex
	log *log.SugaredLogger

	once    sync.Once
	wg      sync.WaitGroup
	closing chan struct{}
}

type dbEntry struct {
	address string

	lastPing time.Time
	lastPong time.Time
}

func NewMapDB(log *log.SugaredLogger) *DB {
	db := &DB{
		m:       make(map[string]*dbEntry),
		log:     log,
		closing: make(chan struct{}),
	}

	return db
}

func (db *DB) Close() {
	db.log.Debugf("closing")

	close(db.closing)
	db.wg.Wait()
}

func (db *DB) ensureExpirer() {
	db.once.Do(func() {
		db.wg.Add(1)
		go db.expirer()
	})
}

func (db *DB) expirer() {
	defer db.wg.Done()

	tick := time.NewTicker(cleanupInterval)
	defer tick.Stop()

	db.expirePeers()
	for {
		select {
		case <-tick.C:
			db.expirePeers()
		case <-db.closing:
			return
		}
	}
}

func (db *DB) expirePeers() {
	threshold := time.Now().Add(-peerExpiration)

	db.mu.Lock()
	for id, entry := range db.m {
		if entry.lastPong.Before(threshold) {
			delete(db.m, id)
		}
	}
	db.mu.Unlock()
}

func (db *DB) getOrSet(id nodeID) *dbEntry {
	entry := db.m[id]
	if entry == nil {
		entry = &dbEntry{}
		db.m[id] = entry
	}
	return entry
}

func (db *DB) UpdateAddress(id nodeID, address string) {
	db.mu.Lock()
	entry := db.getOrSet(id)
	entry.address = address
	db.mu.Unlock()
}

func (db *DB) UpdateLastPing(id nodeID, address string, t time.Time) {
	db.mu.Lock()
	entry := db.getOrSet(id)
	// update address
	entry.address = address

	entry.lastPing = t
	db.mu.Unlock()
}

func (db *DB) UpdateLastPong(id nodeID, address string, t time.Time) {
	db.ensureExpirer()

	db.mu.Lock()
	entry := db.getOrSet(id)
	// update address
	entry.address = address

	entry.lastPong = t
	db.mu.Unlock()
}

func (db *DB) Address(id nodeID) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	entry := db.m[id]
	return entry.address
}

func (db *DB) LastPing(id nodeID) time.Time {
	db.mu.RLock()
	defer db.mu.RUnlock()

	entry := db.m[id]
	return entry.lastPing
}

func (db *DB) LastPong(id nodeID) time.Time {
	db.mu.RLock()
	defer db.mu.RUnlock()

	entry := db.m[id]
	return entry.lastPing
}

func (db *DB) GetRandomID() nodeID {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if len(db.m) == 0 {
		return ""
	}

	i := rand.Intn(len(db.m))
	for k := range db.m {
		if i == 0 {
			return k
		}
		i--
	}

	panic("never")
}
