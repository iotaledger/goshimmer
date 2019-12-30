package gossip

import (
	"errors"

	"github.com/golang/protobuf/proto"
	zL "github.com/iotaledger/autopeering-sim/logger"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/goshimmer/packages/gossip"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/transport"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/events"
	"go.uber.org/zap"
)

const defaultZLC = `{
	"level": "info",
	"development": false,
	"outputPaths": ["./gossip.log"],
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
	debugLevel = "info"
	zLogger    *zap.SugaredLogger
	mgr        *gp.Manager
)

func getTransaction(h []byte) ([]byte, error) {
	log.Info("Retrieving tx:", string(h))
	tx, err := tangle.GetTransaction(string(h))
	if err != nil {
		return []byte{}, err
	}
	if tx == nil {
		return []byte{}, errors.New("Not found")
	}
	pTx := &pb.TransactionRequest{
		Hash: tx.GetBytes(),
	}
	b, _ := proto.Marshal(pTx)
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
			go mgr.AddInbound(ev.Peer)
		}
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Info("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			go mgr.AddOutbound(ev.Peer)
		}
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		log.Info("gossip solid tx", tx.MetaTransaction.GetHash())
		t := &pb.Transaction{
			Body: tx.MetaTransaction.GetBytes(),
		}
		b, err := proto.Marshal(t)
		if err != nil {
			return
		}
		go mgr.SendTransaction(b)
	}))

	gossip.Events.RequestTransaction.Attach(events.NewClosure(func(ev *gossip.RequestTransactionEvent) {
		pTx := &pb.TransactionRequest{}
		proto.Unmarshal(ev.Hash, pTx)
		log.Info("Tx Requested:", string(pTx.Hash))
		go mgr.RequestTransaction(pTx.Hash)
	}))
}
