package autopeering

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/logger"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/autopeering-sim/server"
	"github.com/iotaledger/autopeering-sim/transport"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

var (
	PLUGIN    = node.NewPlugin("Auto Peering", node.Enabled, configure, run)
	close     = make(chan struct{}, 1)
	srv       *server.Server
	Discovery *discover.Protocol
	Selection *selection.Protocol
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

func start() {
	var (
		//listenAddr = "0.0.0.0:14626" //flag.String("addr", "127.0.0.1:14626", "listen address")
		//gossipAddr = "127.0.0.1:14666"
		masterPeer = "" //flag.String("master", "", "master node as 'pubKey@address' where pubKey is in Base64")

		err error
	)

	host := getMyIP()
	apPort := strconv.Itoa(*parameters.PORT.Value)
	gossipPort := strconv.Itoa(*gossip.PORT.Value)
	listenAddr := host + ":" + apPort
	gossipAddr := host + ":" + gossipPort

	logger := logger.NewLogger(defaultZLC, "debug")
	defer func() { _ = logger.Sync() }() // ignore the returned error

	logger.Debug(host)

	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Fatalf("ResolveUDPAddr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()

	var masterPeers []*peer.Peer
	master, err := parseMaster(masterPeer)
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
	local, err := peer.NewLocal(db)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}
	// add a service for the peering
	local.Services()["peering"] = peer.NetworkAddress{Network: "udp", Address: listenAddr}
	// add a service for the gossip
	local.Services()["gossip"] = peer.NetworkAddress{Network: "tcp", Address: gossipAddr}

	Discovery = discover.New(local, discover.Config{
		Log:         logger.Named("disc"),
		MasterPeers: masterPeers,
	})
	Selection = selection.New(local, Discovery, selection.Config{
		Log:          logger.Named("sel"),
		SaltLifetime: selection.DefaultSaltLifetime,
	})

	// start a server doing discovery and peering
	srv = server.Listen(local, trans, logger.Named("srv"), Discovery, Selection)
	defer srv.Close()

	// start the discovery on that connection
	Discovery.Start(srv)
	defer Discovery.Close()

	// start the peering on that connection
	Selection.Start(srv)
	defer Selection.Close()

	id := base64.StdEncoding.EncodeToString(local.PublicKey())
	fmt.Println("Discovery protocol started: ID=" + id + ", address=" + srv.LocalAddr())

	<-close
}

func parseMaster(s string) (*peer.Peer, error) {
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

	return peer.NewPeer(pubKey, parts[1]), nil
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
