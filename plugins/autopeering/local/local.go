package local

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/hive.go/logger"
)

var (
	instance *peer.Local
	once     sync.Once
)

func configureLocal() *peer.Local {
	log := logger.NewLogger("Local")

	ip := net.ParseIP(parameter.NodeConfig.GetString(CFG_ADDRESS))
	if ip == nil {
		log.Fatalf("Invalid IP address: %s", parameter.NodeConfig.GetString(CFG_ADDRESS))
	}
	if ip.IsUnspecified() {
		log.Info("Querying public IP ...")
		myIp, err := getPublicIP(isIPv4(ip))
		if err != nil {
			log.Fatalf("Error querying public IP: %s", err)
		}
		ip = myIp
		log.Infof("Public IP queried: address=%s", ip.String())
	}

	port := strconv.Itoa(parameter.NodeConfig.GetInt(CFG_PORT))

	// create a new local node
	db := peer.NewPersistentDB(log)

	// the private key seed of the current local can be returned the following way:
	// key, _ := db.LocalPrivateKey()
	// fmt.Println(base64.StdEncoding.EncodeToString(ed25519.PrivateKey(key).Seed()))

	// set the private key from the seed provided in the config
	var seed [][]byte
	if parameter.NodeConfig.IsSet(CFG_SEED) {
		str := parameter.NodeConfig.GetString(CFG_SEED)
		bytes, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			log.Fatalf("Invalid seed: %s", err)
		}
		if l := len(bytes); l != ed25519.SeedSize {
			log.Fatalf("Invalid seed length: %d, need %d", l, ed25519.SeedSize)
		}
		seed = append(seed, bytes)
	}

	local, err := peer.NewLocal("udp", net.JoinHostPort(ip.String(), port), db, seed...)
	if err != nil {
		log.Fatalf("Error creating local: %s", err)
	}
	log.Infof("Initialized local: %v", local)

	return local
}

func isIPv4(ip net.IP) bool {
	return ip.To4() != nil
}

func getPublicIP(ipv4 bool) (net.IP, error) {
	var url string
	if ipv4 {
		url = "https://api.ipify.org"
	} else {
		url = "https://api6.ipify.org"
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// the body only consists of the ip address
	ip := net.ParseIP(string(body))
	if ip == nil {
		return nil, fmt.Errorf("not an IP: %s", body)
	}

	return ip, nil
}

func GetInstance() *peer.Local {
	once.Do(func() { instance = configureLocal() })
	return instance
}
