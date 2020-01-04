package autopeering

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/logger"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/autopeering-sim/server"
	"github.com/iotaledger/autopeering-sim/transport"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/pkg/errors"
)

var (
	// Discovery is the peer discovery protocol.
	Discovery *discover.Protocol
	// Selection is the peer selection protocol.
	Selection *selection.Protocol
)

const defaultZLC = `{
	"level": "info",
	"development": false,
	"outputPaths": ["./autopeering.log"],
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

var zLogger = logger.NewLogger(defaultZLC, logLevel)

func configureLocal() {
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

	apPort := strconv.Itoa(parameter.NodeConfig.GetInt(CFG_PORT))
	gossipPort := strconv.Itoa(parameter.NodeConfig.GetInt(gossip.GOSSIP_PORT))

	// create a new local node
	db := peer.NewPersistentDB(zLogger.Named("db"))

	var err error
	local.INSTANCE, err = peer.NewLocal(NETWORK, net.JoinHostPort(ip.String(), apPort), db)
	if err != nil {
		log.Fatalf("NewLocal: %v", err)
	}

	// add a service for the gossip
	if parameter.NodeConfig.GetBool(CFG_SELECTION) {
		err = local.INSTANCE.UpdateService(service.GossipKey, "tcp", net.JoinHostPort(ip.String(), gossipPort))
		if err != nil {
			log.Fatalf("UpdateService: %v", err)
		}
	}
}

func configureAP() {
	masterPeers, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Master peers: %v", masterPeers)

	Discovery = discover.New(local.INSTANCE, discover.Config{
		Log:         zLogger.Named("disc"),
		MasterPeers: masterPeers,
	})

	if parameter.NodeConfig.GetBool(CFG_SELECTION) {
		Selection = selection.New(local.INSTANCE, Discovery, selection.Config{
			Log: zLogger.Named("sel"),
			Param: &selection.Parameters{
				SaltLifetime:    selection.DefaultSaltLifetime,
				RequiredService: []service.Key{service.GossipKey},
			},
		})
	}
}

func start() {
	defer log.Info("Stopping Auto Peering server ... done")

	addr := local.INSTANCE.Services().Get(service.PeeringKey)
	udpAddr, err := net.ResolveUDPAddr(addr.Network(), addr.String())
	if err != nil {
		log.Fatalf("ResolveUDPAddr: %v", err)
	}

	// if the ip is an external ip, set it to unspecified
	if udpAddr.IP.IsGlobalUnicast() {
		if udpAddr.IP.To4() != nil {
			udpAddr.IP = net.IPv4zero
		} else {
			udpAddr.IP = net.IPv6unspecified
		}
	}

	conn, err := net.ListenUDP(addr.Network(), udpAddr)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}

	// use the UDP connection for transport
	trans := transport.Conn(conn, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) })
	defer trans.Close()

	handlers := []server.Handler{Discovery}
	if Selection != nil {
		handlers = append(handlers, Selection)
	}

	// start a server doing discovery and peering
	srv := server.Listen(local.INSTANCE, trans, zLogger.Named("srv"), handlers...)
	defer srv.Close()

	// start the discovery on that connection
	Discovery.Start(srv)
	defer Discovery.Close()

	if Selection != nil {
		// start the peering on that connection
		Selection.Start(srv)
		defer Selection.Close()
	}

	log.Infof("Auto Peering server started: ID=%x, address=%s", local.INSTANCE.ID(), srv.LocalAddr())

	<-daemon.ShutdownSignal
	log.Info("Stopping Auto Peering server ...")
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range parameter.NodeConfig.GetStringSlice(CFG_ENTRY_NODES) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("parseMaster")
		}
		pubKey, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, errors.Wrap(err, "parseMaster")
		}

		services := service.New()
		services.Update(service.PeeringKey, "udp", parts[1])

		result = append(result, peer.NewPeer(pubKey, services))
	}

	return result, nil
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
