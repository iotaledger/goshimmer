package peer

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/logger"
)

const (
	// remove peers from DB, when the last received ping was older than this
	peerExpiration = 24 * time.Hour
	// interval in which expired peers are checked
	cleanupInterval = time.Hour

	// number of peers used for bootstrapping
	seedCount = 10
	// time after which potential seed peers should expire
	seedExpiration = 5 * 24 * time.Hour
)

// DB is the peer database, storing previously seen peers and any collected
// properties of them.
type DB interface {
	// LocalPrivateKey returns the private key stored in the database or creates a new one.
	LocalPrivateKey() (PrivateKey, error)
	// UpdateLocalPrivateKey stores the provided key in the database.
	UpdateLocalPrivateKey(key PrivateKey) error

	// Peer retrieves a peer from the database.
	Peer(id ID) *Peer
	// UpdatePeer updates a peer in the database.
	UpdatePeer(p *Peer) error
	// SeedPeers retrieves random nodes to be used as potential bootstrap peers.
	SeedPeers() []*Peer

	// LastPing returns that property for the given peer ID and address.
	LastPing(id ID, address string) time.Time
	// UpdateLastPing updates that property for the given peer ID and address.
	UpdateLastPing(id ID, address string, t time.Time) error

	// LastPong returns that property for the given peer ID and address.
	LastPong(id ID, address string) time.Time
	// UpdateLastPong updates that property for the given peer ID and address.
	UpdateLastPong(id ID, address string, t time.Time) error

	// Close closes the DB.
	Close()
}

type persistentDB struct {
	db  database.Database
	log *logger.Logger

	closeOnce sync.Once
}

// Keys in the node database.
const (
	dbNodePrefix  = "n:"     // Identifier to prefix node entries with
	dbLocalPrefix = "local:" // Identifier to prefix local entries

	// These fields are stored per ID and address. Use nodeFieldKey to create those keys.
	dbNodePing = "lastping"
	dbNodePong = "lastpong"

	// Local information is keyed by ID only. Use localFieldKey to create those keys.
	dbLocalKey = "key"
)

// NewPersistentDB creates a new persistent DB.
func NewPersistentDB(log *logger.Logger) DB {
	db, err := database.Get(database.DBPrefixAutoPeering, database.GetBadgerInstance())
	if err != nil {
		panic(err)
	}

	pDB := &persistentDB{
		db:  db,
		log: log,
	}
	pDB.start()

	return pDB
}

// Close closes the DB.
func (db *persistentDB) Close() {
	db.closeOnce.Do(func() {
		db.persistSeeds()
	})
}

func (db *persistentDB) start() {
	// get all peers in the DB
	peers := db.getPeers(0)

	for _, p := range peers {
		// if they dont have an associated pong, give them a grace period
		if db.LastPong(p.ID(), p.Address()).Unix() == 0 {
			err := db.setPeerWithTTL(p, cleanupInterval)
			if err != nil {
				db.log.Warnw("database error", "err", err)
			}
		}
	}
}

// persistSeeds assures that potential bootstrap peers are not garbage collected.
func (db *persistentDB) persistSeeds() {
	// randomly select potential bootstrap peers
	peers := randomSubset(db.getPeers(peerExpiration), seedCount)

	for _, p := range peers {
		err := db.setPeerWithTTL(p, seedExpiration)
		if err != nil {
			db.log.Warnw("database error", "err", err)
		}
	}
	db.log.Infof("%d bootstrap peers remain in DB", len(peers))
}

// nodeKey returns the database key for a node record.
func nodeKey(id ID) []byte {
	return append([]byte(dbNodePrefix), id.Bytes()...)
}

func splitNodeKey(key []byte) (id ID, rest []byte) {
	if !bytes.HasPrefix(key, []byte(dbNodePrefix)) {
		return ID{}, nil
	}
	item := key[len(dbNodePrefix):]
	copy(id[:], item[:len(id)])
	return id, item[len(id):]
}

// nodeFieldKey returns the database key for a node metadata field.
func nodeFieldKey(id ID, address string, field string) []byte {
	return bytes.Join([][]byte{nodeKey(id), []byte(address), []byte(field)}, []byte{':'})
}

func localFieldKey(field string) []byte {
	return append([]byte(dbLocalPrefix), []byte(field)...)
}

func parseInt64(blob []byte) int64 {
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

// getInt64 retrieves an integer associated with a particular key.
func (db *persistentDB) getInt64(key []byte) int64 {
	entry, err := db.db.Get(key)
	if err != nil {
		return 0
	}
	return parseInt64(entry.Value)
}

// setInt64 stores an integer in the given key.
func (db *persistentDB) setInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.db.Set(database.Entry{Key: key, Value: blob, TTL: peerExpiration})
}

// LocalPrivateKey returns the private key stored in the database or creates a new one.
func (db *persistentDB) LocalPrivateKey() (PrivateKey, error) {
	entry, err := db.db.Get(localFieldKey(dbLocalKey))
	if err == database.ErrKeyNotFound {
		key, genErr := generatePrivateKey()
		if genErr == nil {
			err = db.UpdateLocalPrivateKey(key)
		}
		return key, err
	}
	if err != nil {
		return nil, err
	}
	return PrivateKey(entry.Value), nil
}

// UpdateLocalPrivateKey stores the provided key in the database.
func (db *persistentDB) UpdateLocalPrivateKey(key PrivateKey) error {
	return db.db.Set(database.Entry{Key: localFieldKey(dbLocalKey), Value: []byte(key)})
}

// LastPing returns that property for the given peer ID and address.
func (db *persistentDB) LastPing(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeFieldKey(id, address, dbNodePing)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *persistentDB) UpdateLastPing(id ID, address string, t time.Time) error {
	return db.setInt64(nodeFieldKey(id, address, dbNodePing), t.Unix())
}

// LastPing returns that property for the given peer ID and address.
func (db *persistentDB) LastPong(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeFieldKey(id, address, dbNodePong)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *persistentDB) UpdateLastPong(id ID, address string, t time.Time) error {
	return db.setInt64(nodeFieldKey(id, address, dbNodePong), t.Unix())
}

func (db *persistentDB) setPeerWithTTL(p *Peer, ttl time.Duration) error {
	data, err := p.Marshal()
	if err != nil {
		return err
	}
	return db.db.Set(database.Entry{Key: nodeKey(p.ID()), Value: data, TTL: ttl})
}

func (db *persistentDB) UpdatePeer(p *Peer) error {
	return db.setPeerWithTTL(p, peerExpiration)
}

func parsePeer(data []byte) *Peer {
	p, err := Unmarshal(data)
	if err != nil {
		return nil
	}
	return p
}

func (db *persistentDB) Peer(id ID) *Peer {
	data, err := db.db.Get(nodeKey(id))
	if err != nil {
		return nil
	}
	return parsePeer(data.Value)
}

func randomSubset(peers []*Peer, m int) []*Peer {
	if len(peers) <= m {
		return peers
	}

	result := make([]*Peer, 0, m)
	for i, p := range peers {
		if rand.Intn(len(peers)-i) < m-len(result) {
			result = append(result, p)
		}
	}
	return result
}

func (db *persistentDB) getPeers(maxAge time.Duration) []*Peer {
	peers := make([]*Peer, 0)
	now := time.Now()

	err := db.db.StreamForEachPrefix([]byte(dbNodePrefix), func(entry database.Entry) error {
		id, rest := splitNodeKey(entry.Key)
		if len(rest) > 0 {
			return nil
		}

		p := parsePeer(entry.Value)
		if p == nil || p.ID() != id {
			return nil
		}
		if maxAge > 0 && now.Sub(db.LastPong(p.ID(), p.Address())) > maxAge {
			return nil
		}

		peers = append(peers, p)
		return nil
	})
	if err != nil {
		return []*Peer{}
	}
	return peers
}

// SeedPeers retrieves random nodes to be used as potential bootstrap peers.
func (db *persistentDB) SeedPeers() []*Peer {
	// get all stored peers and select subset
	peers := db.getPeers(0)

	seeds := randomSubset(peers, seedCount)
	db.log.Infof("%d potential bootstrap peers restored form DB", len(seeds))

	return seeds
}
