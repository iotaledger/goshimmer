package peerstorage

import (
	"bytes"
	"github.com/iotaledger/hive.go/logger"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

const peerDbName string = "peers"

var log = logger.NewLogger("Autopeering-Peerstorage")

var peerDb database.Database
var once sync.Once

func initDb() {
	db, err := database.Get(peerDbName)
	if err != nil {
		panic(err)
	}

	peerDb = db
}

func getDb() database.Database {
	once.Do(initDb)

	return peerDb
}

func storePeer(p *peer.Peer) {
	err := getDb().Set(p.GetIdentity().Identifier, p.Marshal())
	if err != nil {
		panic(err)
	}
}

func removePeer(p *peer.Peer) {
	err := getDb().Delete(p.GetIdentity().Identifier)
	if err != nil {
		panic(err)
	}
}

func loadPeers(plugin *node.Plugin) {
	var count int

	err := getDb().ForEach(func(key []byte, value []byte) {
		peer, err := peer.Unmarshal(value)
		if err != nil {
			panic(err)
		}
		// the peers are stored by identifier in the db
		if !bytes.Equal(key, peer.GetIdentity().Identifier) {
			panic("Invalid item in '" + peerDbName + "' database")
		}

		knownpeers.INSTANCE.AddOrUpdate(peer)
		count++
		log.Debugf("Added stored peer: %s / %s", peer.GetAddress().String(), peer.GetIdentity().StringIdentifier)
	})
	if err != nil {
		panic(err)
	}

	log.Infof("Restored %d peers from database", count)
}

func Configure(plugin *node.Plugin) {
	// do not store the entry nodes by ignoring all peers currently contained in konwnpeers
	// add peers from db
	loadPeers(plugin)

	// subscribe to all known peers' events
	knownpeers.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
		storePeer(p)
	}))
	knownpeers.INSTANCE.Events.Update.Attach(events.NewClosure(func(p *peer.Peer) {
		storePeer(p)
	}))
	knownpeers.INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
		removePeer(p)
	}))
}
