package peer

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var log = logger.NewExampleLogger("peer")

func TestMapDBPing(t *testing.T) {
	p := newTestPeer()
	db := NewMemoryDB(log)

	now := time.Now()
	err := db.UpdateLastPing(p.ID(), p.Address(), now)
	require.NoError(t, err)

	assert.Equal(t, now.Unix(), db.LastPing(p.ID(), p.Address()).Unix())
}

func TestMapDBPong(t *testing.T) {
	p := newTestPeer()
	db := NewMemoryDB(log)

	now := time.Now()
	err := db.UpdateLastPong(p.ID(), p.Address(), now)
	require.NoError(t, err)

	assert.Equal(t, now.Unix(), db.LastPong(p.ID(), p.Address()).Unix())
}

func TestMapDBPeer(t *testing.T) {
	p := newTestPeer()
	db := NewMemoryDB(log)

	err := db.UpdatePeer(p)
	require.NoError(t, err)

	assert.Equal(t, p, db.Peer(p.ID()))
}

func TestMapDBSeedPeers(t *testing.T) {
	p := newTestPeer()
	db := NewMemoryDB(log)

	require.NoError(t, db.UpdatePeer(p))
	require.NoError(t, db.UpdateLastPong(p.ID(), p.Address(), time.Now()))

	peers := db.SeedPeers()
	assert.ElementsMatch(t, []*Peer{p}, peers)
}

func TestMapDBLocal(t *testing.T) {
	db := NewMemoryDB(log)

	l1, err := NewLocal(testNetwork, testAddress, db)
	require.NoError(t, err)
	assert.Equal(t, len(l1.PublicKey()), ed25519.PublicKeySize)

	l2, err := NewLocal(testNetwork, testAddress, db)
	require.NoError(t, err)

	assert.Equal(t, l1, l2)
}
