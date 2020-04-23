package local

import (
	"crypto/ed25519"
	"encoding/base64"
	"net"
	"strings"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
)

var (
	instance *peer.Local
	once     sync.Once
)

func configureLocal() *peer.Local {
	log := logger.NewLogger("Local")

	var peeringIP net.IP
	if str := config.Node.GetString(CfgExternal); strings.ToLower(str) == "auto" {
		// let the autopeering discover the IP
		peeringIP = net.IPv4zero
	} else {
		peeringIP = net.ParseIP(str)
		if peeringIP == nil {
			log.Fatalf("Invalid IP address (%s): %s", CfgExternal, str)
		}
		if !peeringIP.IsGlobalUnicast() {
			log.Warnf("IP is not a global unicast address: %s", peeringIP.String())
		}
	}

	peeringPort := config.Node.GetInt(CfgPort)
	if 0 > peeringPort || peeringPort > 65535 {
		log.Fatalf("Invalid port number (%s): %d", CfgPort, peeringPort)
	}

	// announce the peering service
	services := service.New()
	services.Update(service.PeeringKey, "udp", peeringPort)

	// set the private key from the seed provided in the config
	var seed [][]byte
	if str := config.Node.GetString(CfgSeed); str != "" {
		bytes, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			log.Fatalf("Invalid %s: %s", CfgSeed, err)
		}
		if l := len(bytes); l != ed25519.SeedSize {
			log.Fatalf("Invalid %s length: %d, need %d", CfgSeed, l, ed25519.SeedSize)
		}
		seed = append(seed, bytes)
	}
	badgerDB, err := database.Get(database.DBPrefixAutoPeering, database.GetBadgerInstance())
	if err != nil {
		log.Fatalf("Error loading DB: %s", err)
	}
	peerDB, err := peer.NewDB(badgerDB)
	if err != nil {
		log.Fatalf("Error creating peer DB: %s", err)
	}

	// the private key seed of the current local can be returned the following way:
	// key, _ := peerDB.LocalPrivateKey()
	// fmt.Println(base64.StdEncoding.EncodeToString(ed25519.PrivateKey(key).Seed()))

	local, err := peer.NewLocal(peeringIP, services, peerDB, seed...)
	if err != nil {
		log.Fatalf("Error creating local: %s", err)
	}
	log.Infof("Initialized local: %v", local)

	return local
}

// GetInstance returns the instance of the local peer.
func GetInstance() *peer.Local {
	once.Do(func() { instance = configureLocal() })
	return instance
}
