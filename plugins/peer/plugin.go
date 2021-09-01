package peer

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/mr-tron/base58"
	"go.uber.org/dig"

	databasePkg "github.com/iotaledger/goshimmer/packages/database"
)

// PluginName is the name of the Peer plugin.
const PluginName = "Peer"

var (
	// Plugin is the plugin instance of the Peer plugin.
	Plugin *node.Plugin
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Enabled)

	Plugin.Events.Init.Attach(events.NewClosure(func(_ *node.Plugin, container *dig.Container) {
		if err := container.Provide(configureLocal); err != nil {
			Plugin.Panic(err)
		}
	}))
}

// instantiates a local instance.
func configureLocal(kvStore kvstore.KVStore) *peer.Local {
	log := logger.NewLogger("Local")

	var peeringIP net.IP
	if strings.ToLower(Parameters.ExternalAddress) == "auto" {
		// let the autopeering discover the IP
		peeringIP = net.IPv4zero
	} else {
		peeringIP = net.ParseIP(Parameters.ExternalAddress)
		if peeringIP == nil {
			log.Fatalf("Invalid IP address: %s", Parameters.ExternalAddress)
		}
		if !peeringIP.IsGlobalUnicast() {
			log.Warnf("IP is not a global unicast address: %s", peeringIP.String())
		}
	}

	// set the private key from the seed provided in the config
	var seed [][]byte
	if Parameters.Seed != "" {
		var bytes []byte
		var err error

		if strings.HasPrefix(Parameters.Seed, "base58:") {
			bytes, err = base58.Decode(Parameters.Seed[7:])
		} else if strings.HasPrefix(Parameters.Seed, "base64:") {
			bytes, err = base64.StdEncoding.DecodeString(Parameters.Seed[7:])
		} else {
			err = fmt.Errorf("neither base58 nor base64 prefix provided")
		}

		if err != nil {
			log.Fatalf("Invalid seed: %s", err)
		}
		if l := len(bytes); l != ed25519.SeedSize {
			log.Fatalf("Invalid seed length: %d, need %d", l, ed25519.SeedSize)
		}
		seed = append(seed, bytes)
	}
	peerDB, err := peer.NewDB(kvStore.WithRealm([]byte{databasePkg.PrefixAutoPeering}))
	if err != nil {
		log.Fatalf("Error creating peer DB: %s", err)
	}
	if peerDB == nil {
		log.Warnf("nil peerDB")
	}

	// the private key seed of the current local can be returned the following way:
	// key, _ := peerDB.LocalPrivateKey()
	// fmt.Printf("Seed: base58:%s\n", key.Seed().String())

	// TODO: remove requirement for PeeringKey in hive.go
	services := service.New()
	services.Update(service.PeeringKey, "dummy", 0)

	local, err := peer.NewLocal(peeringIP, services, peerDB, seed...)
	if err != nil {
		log.Fatalf("Error creating local: %s", err)
	}
	log.Infof("Initialized local: %v", local)

	return local
}
