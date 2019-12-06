package gossip

import (
	"github.com/golang/protobuf/proto"
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

var (
	zLogger            *zap.SugaredLogger
	mgr                *gp.Manager
	SendTransaction    = mgr.SendTransaction
	RequestTransaction = mgr.RequestTransaction
	AddInbound         = mgr.AddInbound
	AddOutbound        = mgr.AddOutbound
	DropNeighbor       = mgr.DropNeighbor
)

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	zLogger = l.Sugar()
}

func getTransaction(h []byte) ([]byte, error) {
	tx := &pb.TransactionRequest{
		Hash: []byte("testTx"),
	}
	b, _ := proto.Marshal(tx)
	return b, nil
}

func configureGossip() {
	defer func() { _ = zLogger.Sync() }() // ignore the returned error

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
		mgr.DropNeighbor(ev.DroppedID)
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
