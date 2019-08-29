package discover

import (
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wollac/autopeering/id"
	log "go.uber.org/zap"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour

	keySeparator = ":"
)

type DB struct {
	mutex sync.RWMutex
	m     map[string]dbPeer

	log *log.SugaredLogger

	once    sync.Once
	wg      sync.WaitGroup
	closing chan struct{}
}

type dbPeer struct {
	lastPing, lastPong int64
}

func NewMapDB(log *log.SugaredLogger) *DB {
	db := &DB{
		m:       make(map[string]dbPeer),
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
	threshold := time.Now().Add(-peerExpiration).Unix()

	db.mutex.Lock()
	for key, entry := range db.m {
		if entry.lastPong <= threshold {
			delete(db.m, key)
		}
	}
	db.mutex.Unlock()
}

func getDBKey(p *Peer) string {
	return strings.Join([]string{p.Identity.IDKey(), p.Address}, keySeparator)
}

func splitDBKey(key string) (id string, address string) {
	parts := strings.SplitN(key, keySeparator, 2)
	return parts[0], parts[1]
}

func (db *DB) validIndices(maxAge time.Duration) []int {
	var (
		threshold = time.Now().Add(-maxAge).Unix()
		indices   = make([]int, 0, len(db.m))
		i         int
	)

	for _, v := range db.m {
		if v.lastPong >= threshold {
			indices = append(indices, i)
		}
	}

	return indices
}

func (db *DB) RandomPeers(n int, maxAge time.Duration) []*Peer {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	indices := db.validIndices(maxAge)
	rand.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})
	if n > len(indices) {
		n = len(indices)
	}
	indices = indices[:n]
	sort.Ints(indices)

	var (
		peers   = make([]*Peer, 0, n)
		i, seek int
	)
	for k := range db.m {
		if len(peers) == n {
			break
		}
		if indices[seek] == i {
			seek++

			idKey, address := splitDBKey(k)
			identity, err := id.FromIDKey(idKey)
			if err != nil {
				db.log.Error("invalid DB entry: ", err)
				continue
			}
			peers = append(peers, NewPeer(identity, address))
		}
		i++
	}

	return peers
}

func (db *DB) LastPing(p *Peer) time.Time {
	db.ensureExpirer()
	key := getDBKey(p)

	db.mutex.RLock()
	entry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(entry.lastPing, 0)
}

func (db *DB) UpdateLastPing(p *Peer, t time.Time) {
	key := getDBKey(p)

	db.mutex.Lock()
	entry := db.m[key]
	entry.lastPing = t.Unix()
	db.m[key] = entry
	db.mutex.Unlock()
}

func (db *DB) LastPong(p *Peer) time.Time {
	db.ensureExpirer()
	key := getDBKey(p)

	db.mutex.RLock()
	entry := db.m[key]
	db.mutex.RUnlock()

	return time.Unix(entry.lastPong, 0)
}

func (db *DB) UpdateLastPong(p *Peer, t time.Time) {
	key := getDBKey(p)

	db.mutex.Lock()
	entry := db.m[key]
	entry.lastPong = t.Unix()
	db.m[key] = entry
	db.mutex.Unlock()
}
