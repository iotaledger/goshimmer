package gossip

import (
	"github.com/golang/protobuf/proto"
	zL "github.com/iotaledger/autopeering-sim/logger"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/autopeering-sim/selection"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/transport"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/events"
	"go.uber.org/zap"
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

var (
	debugLevel         = "debug"
	zLogger            *zap.SugaredLogger
	mgr                *gp.Manager
	SendTransaction    = mgr.SendTransaction
	RequestTransaction = mgr.RequestTransaction
	AddInbound         = mgr.AddInbound
	AddOutbound        = mgr.AddOutbound
	DropNeighbor       = mgr.DropNeighbor
)

func getTransaction(h []byte) ([]byte, error) {
	tx := &pb.TransactionRequest{
		Hash: []byte("testTx"),
	}
	b, _ := proto.Marshal(tx)
	return b, nil
}

func configureGossip() {
	zLogger = zL.NewLogger(defaultZLC, debugLevel)

	trans, err := transport.Listen(local.INSTANCE, zLogger)
	if err != nil {
		log.Fatal(err)
	}

	mgr = gp.NewManager(trans, zLogger, getTransaction)
	log.Info("Gossip started @", trans.LocalAddr().String())
}

func configureEvents() {

	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Info("neighbor removed: " + ev.DroppedID.String())
		go mgr.DropNeighbor(ev.DroppedID)
	}))

	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Info("accepted neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			//address, port, _ := net.SplitHostPort(ev.Peer.Services().Get(service.GossipKey).String())
			go mgr.AddInbound(ev.Peer)
		}
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Info("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			//address, port, _ := net.SplitHostPort(ev.Peer.Services().Get(service.GossipKey).String())
			go mgr.AddOutbound(ev.Peer)
		}
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *meta_transaction.MetaTransaction) {
		log.Info("Tx solidified")
		t := &pb.Transaction{
			Body: tx.GetBytes(),
		}
		b, err := proto.Marshal(t)
		if err != nil {
			return
		}
		go SendTransaction(b)
	}))
}
