package local

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/logger"
	"github.com/mr-tron/base58"

	databasePkg "github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/database"
)

var (
	instance *peer.Local
	once     sync.Once
)

func configureLocal() *peer.Local {
	log := logger.NewLogger("Local")

	var peeringIP net.IP
	if strings.ToLower(ParametersNetwork.ExternalAddress) == "auto" {
		// let the autopeering discover the IP
		peeringIP = net.IPv4zero
	} else {
		peeringIP = net.ParseIP(ParametersNetwork.ExternalAddress)
		if peeringIP == nil {
			log.Fatalf("Invalid IP address: %s", ParametersNetwork.ExternalAddress)
		}
		if !peeringIP.IsGlobalUnicast() {
			log.Warnf("IP is not a global unicast address: %s", peeringIP.String())
		}
	}

	if ParametersLocal.Port < 0 || ParametersLocal.Port > 65535 {
		log.Fatalf("Invalid port number: %d", ParametersLocal.Port)
	}

	// announce the peering service
	services := service.New()
	services.Update(service.PeeringKey, "udp", ParametersLocal.Port)

	// set the private key from the seed provided in the config
	var seed [][]byte
	if ParametersLocal.Seed != "" {
		var bytes []byte
		var err error

		if strings.HasPrefix(ParametersLocal.Seed, "base58:") {
			bytes, err = base58.Decode(ParametersLocal.Seed[7:])
		} else if strings.HasPrefix(ParametersLocal.Seed, "base64:") {
			bytes, err = base64.StdEncoding.DecodeString(ParametersLocal.Seed[7:])
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
	peerDB, err := peer.NewDB(database.StoreRealm([]byte{databasePkg.PrefixAutoPeering}))
	if err != nil {
		log.Fatalf("Error creating peer DB: %s", err)
	}

	// the private key seed of the current local can be returned the following way:
	// key, _ := peerDB.LocalPrivateKey()
	// fmt.Printf("Seed: base58:%s\n", key.Seed().String())

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
