package peer

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
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

// Database defines the database functionality required by DB.
type Database interface {
	Get(database.Key) (database.Entry, error)
	Set(database.Entry) error
	ForEachPrefix(database.KeyPrefix, func(database.Entry) bool) error
}

// DB is the peer database, storing previously seen peers and any collected properties of them.
type DB struct {
	db Database
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

// NewDB creates a new peer database.
func NewDB(db Database) (*DB, error) {
	pDB := &DB{
		db: db,
	}
	err := pDB.init()
	if err != nil {
		return nil, err
	}
	return pDB, nil
}

func (db *DB) init() error {
	// get all peers in the DB
	peers := db.getPeers(0)

	for _, p := range peers {
		// if they dont have an associated pong, give them a grace period
		if db.LastPong(p.ID(), p.Address()).Unix() == 0 {
			if err := db.setPeerWithTTL(p, cleanupInterval); err != nil {
				return err
			}
		}
	}
	return nil
}

// nodeKey returns the database key for a node record.
func nodeKey(id ID) []byte {
	return append([]byte(dbNodePrefix), id.Bytes()...)
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
func (db *DB) getInt64(key []byte) int64 {
	entry, err := db.db.Get(key)
	if err != nil {
		return 0
	}
	return parseInt64(entry.Value)
}

// setInt64 stores an integer in the given key.
func (db *DB) setInt64(key []byte, n int64) error {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	return db.db.Set(database.Entry{Key: key, Value: blob, TTL: peerExpiration})
}

// LocalPrivateKey returns the private key stored in the database or creates a new one.
func (db *DB) LocalPrivateKey() (PrivateKey, error) {
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
func (db *DB) UpdateLocalPrivateKey(key PrivateKey) error {
	return db.db.Set(database.Entry{Key: localFieldKey(dbLocalKey), Value: []byte(key)})
}

// LastPing returns that property for the given peer ID and address.
func (db *DB) LastPing(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeFieldKey(id, address, dbNodePing)), 0)
}

// UpdateLastPing updates that property for the given peer ID and address.
func (db *DB) UpdateLastPing(id ID, address string, t time.Time) error {
	return db.setInt64(nodeFieldKey(id, address, dbNodePing), t.Unix())
}

// LastPong returns that property for the given peer ID and address.
func (db *DB) LastPong(id ID, address string) time.Time {
	return time.Unix(db.getInt64(nodeFieldKey(id, address, dbNodePong)), 0)
}

// UpdateLastPong updates that property for the given peer ID and address.
func (db *DB) UpdateLastPong(id ID, address string, t time.Time) error {
	return db.setInt64(nodeFieldKey(id, address, dbNodePong), t.Unix())
}

func (db *DB) setPeerWithTTL(p *Peer, ttl time.Duration) error {
	data, err := p.Marshal()
	if err != nil {
		return err
	}
	return db.db.Set(database.Entry{Key: nodeKey(p.ID()), Value: data, TTL: ttl})
}

// UpdatePeer updates a peer in the database.
func (db *DB) UpdatePeer(p *Peer) error {
	return db.setPeerWithTTL(p, peerExpiration)
}

func parsePeer(data []byte) *Peer {
	p, err := Unmarshal(data)
	if err != nil {
		return nil
	}
	return p
}

// Peer retrieves a peer from the database.
func (db *DB) Peer(id ID) *Peer {
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

func (db *DB) getPeers(maxAge time.Duration) (peers []*Peer) {
	now := time.Now()
	err := db.db.ForEachPrefix([]byte(dbNodePrefix), func(entry database.Entry) bool {
		var id ID
		if len(entry.Key) != len(id) {
			return false
		}
		copy(id[:], entry.Key)

		p := parsePeer(entry.Value)
		if p == nil || p.ID() != id {
			return false
		}
		if maxAge > 0 && now.Sub(db.LastPong(p.ID(), p.Address())) > maxAge {
			return false
		}

		peers = append(peers, p)
		return false
	})
	if err != nil {
		return nil
	}
	return peers
}

// SeedPeers retrieves random nodes to be used as potential bootstrap peers.
func (db *DB) SeedPeers() []*Peer {
	// get all stored peers and select subset
	peers := db.getPeers(0)

	return randomSubset(peers, seedCount)
}

// PersistSeeds assures that potential bootstrap peers are not garbage collected.
// It returns the number of peers that have been persisted.
func (db *DB) PersistSeeds() int {
	// randomly select potential bootstrap peers
	peers := randomSubset(db.getPeers(peerExpiration), seedCount)

	for i, p := range peers {
		err := db.setPeerWithTTL(p, seedExpiration)
		if err != nil {
			return i
		}
	}
	return len(peers)
}
