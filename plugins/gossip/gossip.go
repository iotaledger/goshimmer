package gossip

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"golang.org/x/xerrors"

	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// ErrMessageNotFound is returned when a message could not be found in the Tangle.
var ErrMessageNotFound = errors.New("message not found")

var (
	mgr     *gossip.Manager
	mgrOnce sync.Once
)

// Manager returns the manager instance of the gossip plugin.
func Manager() *gossip.Manager {
	mgrOnce.Do(createManager)
	return mgr
}

func createManager() {
	// assure that the logger is available
	log := logger.NewLogger(PluginName)

	// announce the gossip service
	gossipPort := config.Node().Int(CfgGossipPort)
	if !netutil.IsValidPort(gossipPort) {
		log.Fatalf("Invalid port number (%s): %d", CfgGossipPort, gossipPort)
	}

	lPeer := local.GetInstance()
	if err := lPeer.UpdateService(service.GossipKey, "tcp", gossipPort); err != nil {
		log.Fatalf("could not update services: %s", err)
	}
	mgr = gossip.NewManager(lPeer, loadMessage, log)
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + PluginName + " ... done")

	lPeer := local.GetInstance()

	// use the port of the gossip service
	gossipEndpoint := lPeer.Services().Get(service.GossipKey)

	// resolve the bind address
	address := net.JoinHostPort(config.Node().String(local.CfgBind), strconv.Itoa(gossipEndpoint.Port()))
	localAddr, err := net.ResolveTCPAddr(gossipEndpoint.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CfgBind, err)
	}

	listener, err := net.ListenTCP(gossipEndpoint.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer listener.Close()

	srv := server.ServeTCP(lPeer, listener, log)
	defer srv.Close()

	mgr.Start(srv)
	defer mgr.Close()

	// trigger start of the autopeering selection
	go func() { autopeering.StartSelection() }()

	addNeighborsFromConfigToManager(mgr)

	log.Infof("%s started: age-threshold=%v bind-address=%s", PluginName, ageThreshold, localAddr.String())

	<-shutdownSignal
	log.Info("Stopping " + PluginName + " ...")

	// assure that the autopeering selection is always stopped before the gossip manager
	autopeering.Selection().Close()
}

func addNeighborsFromConfigToManager(mgr *gossip.Manager) {
	manualNeighbors, err := getNeighborsFromConfig()
	if err != nil {
		log.Errorw("Failed to get manual neighbors from the config file, continuing without them...", "err", err)
	} else if len(manualNeighbors) != 0 {
		log.Infow("Pass manual neighbors list to gossip layer", "neighbors", manualNeighbors)
		for _, neighbor := range manualNeighbors {
			if err := mgr.AddOutbound(neighbor, gossip.NeighborsGroupManual); err != nil {
				log.Errorw("Gossip failed to add manual neighbor, skipping that neighbor", "err", err)
			}
		}
	}
}

func getNeighborsFromConfig() ([]*peer.Peer, error) {
	rawMap := config.Node().Get(CfgGossipManualNeighbors)
	// This is a hack to transform a map from config into peer.Peer struct.
	jsonData, err := json.Marshal(rawMap)
	if err != nil {
		return nil, xerrors.Errorf("can't marshal neighbors map from config into json data: %w", err)
	}
	var peers []*peer.Peer
	if err := json.Unmarshal(jsonData, &peers); err != nil {
		return nil, xerrors.Errorf("can't parse neighbors from json: %w", err)
	}
	return peers, nil
}

// loads the given message from the message layer and returns it or an error if not found.
func loadMessage(msgID tangle.MessageID) ([]byte, error) {
	cachedMessage := messagelayer.Tangle().Storage.Message(msgID)
	defer cachedMessage.Release()
	if !cachedMessage.Exists() {
		return nil, ErrMessageNotFound
	}
	msg := cachedMessage.Unwrap()
	return msg.Bytes(), nil
}

// requestedMessages represents a list of requested messages that will not be gossiped.
type requestedMessages struct {
	sync.Mutex

	msgs map[tangle.MessageID]types.Empty
}

func newRequestedMessages() *requestedMessages {
	return &requestedMessages{
		msgs: make(map[tangle.MessageID]types.Empty),
	}
}

func (r *requestedMessages) append(msgID tangle.MessageID) {
	r.Lock()
	defer r.Unlock()

	r.msgs[msgID] = types.Void
}

func (r *requestedMessages) delete(msgID tangle.MessageID) (deleted bool) {
	r.Lock()
	defer r.Unlock()

	if _, exist := r.msgs[msgID]; exist {
		delete(r.msgs, msgID)
		return true
	}

	return false
}
