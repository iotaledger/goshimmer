package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/logger"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/autopeering-sim/server"
	"github.com/iotaledger/autopeering-sim/transport"
	"github.com/pkg/errors"
)

const defaultZLC = `{
	"level": "info",
	"development": false,
	"outputPaths": ["stdout"],
	"errorOutputPaths": ["stderr"],
	"encoding": "console",
	"encoderConfig": {
	  "timeKey": "ts",
	  "levelKey": "level",
	  "nameKey": "logger",
	  "callerKey": "caller",
	  "messageKey": "msg",
	  "stacktraceKey": "stacktrace",
	  "lineEnding": "",
	  "levelEncoder": "",
	  "timeEncoder": "iso8601",
	  "durationEncoder": "",
	  "callerEncoder": ""
	}
  }`

func waitInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func parseMaster(network string, s string) (*peer.Peer, error) {
	if len(s) == 0 {
		return nil, nil
	}

	parts := strings.Split(s, "@")
	if len(parts) != 2 {
		return nil, errors.New("parseMaster")
	}
	pubKey, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.Wrap(err, "parseMaster")
	}

	services := service.New()
	services.Update(service.PeeringKey, network, parts[1])

	return peer.NewPeer(pubKey, services), nil
}

func main() {
	var (
		listenAddr = flag.String("addr", "127.0.0.1:14626", "listen address")
		masterPeer = flag.String("master", "", "master node as 'pubKey@address' where pubKey is in Base64")

		err error
	)
	flag.Parse()

	externalAddr := *listenAddr
	if host, port, _ := net.SplitHostPort(*listenAddr); host == "0.0.0.0" {
		externalAddr = getMyIP() + ":" + port
	}

	logger := logger.NewLogger(defaultZLC, "debug")
	defer func() { _ = logger.Sync() }() // ignore the returned error

	addr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		log.Fatalf("ResolveUDPAddr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()

	var masterPeers []*peer.Peer
	master, err := parseMaster("udp", *masterPeer)
	if err != nil {
		log.Printf("Ignoring master: %v\n", err)
	} else if master != nil {
		masterPeers = []*peer.Peer{master}
	}

	// use the UDP connection for transport
	trans := transport.Conn(conn, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) })
	defer trans.Close()

	// create a new local node
	db := peer.NewPersistentDB(logger.Named("db"))
	defer db.Close()
	local, err := peer.NewLocal(trans.LocalAddr().Network(), externalAddr, db)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}

	discovery := discover.New(local, discover.Config{
		Log:         logger.Named("disc"),
		MasterPeers: masterPeers,
	})
	selection := selection.New(local, discovery, selection.Config{
		Log: logger.Named("sel"),
	})

	// start a server doing discovery and peering
	srv := server.Listen(local, trans, logger.Named("srv"), discovery, selection)
	defer srv.Close()

	// start the discovery on that connection
	discovery.Start(srv)
	defer discovery.Close()

	// start the peering on that connection
	selection.Start(srv)
	defer selection.Close()

	pubKey := base64.StdEncoding.EncodeToString(local.PublicKey())
	fmt.Println("Discovery protocol started: pubKey=" + pubKey + ", address=" + srv.LocalAddr())
	fmt.Println("Hit Ctrl+C to exit")

	// wait for Ctrl+c
	waitInterrupt()
}

func getMyIP() string {
	url := "https://api.ipify.org?format=text"
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", ip)
}
