package peer

import (
	"crypto/ed25519"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/database/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeerDB(t *testing.T) {
	db, err := NewDB(mapdb.NewMapDB())
	require.NoError(t, err)
	p := newTestPeer("test")

	t.Run("LocalPrivateKey", func(t *testing.T) {
		key, err := db.LocalPrivateKey()
		require.NoError(t, err)

		assert.EqualValues(t, ed25519.PrivateKeySize, len(key))
		assert.EqualValues(t, ed25519.PublicKeySize, len(key.Public()))

		err = db.UpdateLocalPrivateKey(key)
		require.NoError(t, err)

		key2, err := db.LocalPrivateKey()
		require.NoError(t, err)

		assert.Equal(t, key, key2)
	})

	t.Run("Peer", func(t *testing.T) {
		err := db.UpdatePeer(p)
		require.NoError(t, err)

		assert.Equal(t, p, db.Peer(p.ID()))
	})

	t.Run("LastPing", func(t *testing.T) {
		now := time.Now()
		err := db.UpdateLastPing(p.ID(), p.Address(), now)
		require.NoError(t, err)

		assert.Equal(t, now.Unix(), db.LastPing(p.ID(), p.Address()).Unix())
	})

	t.Run("LastPong", func(t *testing.T) {
		now := time.Now()
		err := db.UpdateLastPong(p.ID(), p.Address(), now)
		require.NoError(t, err)

		assert.Equal(t, now.Unix(), db.LastPong(p.ID(), p.Address()).Unix())
	})

	t.Run("getPeers", func(t *testing.T) {
		time.Sleep(time.Second) // let the old peers expire

		newPeer := newTestPeer("new")
		assert.NoError(t, db.UpdatePeer(newPeer))
		assert.NoError(t, db.UpdateLastPong(newPeer.ID(), newPeer.Address(), time.Now()))

		peers := db.getPeers(time.Second)
		assert.ElementsMatch(t, []*Peer{newPeer}, peers)
	})

	t.Run("SeedPeers", func(t *testing.T) {
		for i := 0; i < seedCount+1; i++ {
			p := newTestPeer(fmt.Sprintf("SeedPeers%0d", i))
			assert.NoError(t, db.UpdatePeer(p))
			assert.NoError(t, db.UpdateLastPong(p.ID(), p.Address(), time.Now()))
		}

		peers := db.SeedPeers()
		assert.EqualValues(t, seedCount, len(peers))
	})

	t.Run("PersistSeeds", func(t *testing.T) {
		for i := 0; i < seedCount+1; i++ {
			p := newTestPeer(fmt.Sprintf("PersistSeeds%0d", i))
			assert.NoError(t, db.UpdatePeer(p))
			assert.NoError(t, db.UpdateLastPong(p.ID(), p.Address(), time.Now()))
		}

		count := db.PersistSeeds()
		assert.EqualValues(t, seedCount, count)
	})
}
