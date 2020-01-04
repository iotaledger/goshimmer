package local

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/hive.go/parameter"
	"go.uber.org/zap"
)

var (
	instance *peer.Local
	once     sync.Once
)

func configureLocal() *peer.Local {
	ip := net.ParseIP(parameter.NodeConfig.GetString(CFG_ADDRESS))
	if ip == nil {
		log.Fatalf("Invalid IP address: %s", parameter.NodeConfig.GetString(CFG_ADDRESS))
	}
	if ip.IsUnspecified() {
		myIp, err := getMyIP()
		if err != nil {
			log.Fatalf("Could not query public IP: %v", err)
		}
		ip = myIp
	}

	port := strconv.Itoa(parameter.NodeConfig.GetInt(CFG_PORT))

	// create a new local node
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Could not create logger: %v", err)
	}
	db := peer.NewPersistentDB(logger.Named("db").Sugar())

	local, err := peer.NewLocal("udp", net.JoinHostPort(ip.String(), port), db)
	if err != nil {
		log.Fatalf("NewLocal: %v", err)
	}

	return local
}

func getMyIP() (net.IP, error) {
	url := "https://api.ipify.org?format=text"
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
