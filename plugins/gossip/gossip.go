package gossip

import (

	"github.com/iotaledger/goshimmer/packages/gossip/transport"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	gp "github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"go.uber.org/zap"
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/hive.go/events"
)

var (
	zLogger *zap.SugaredLogger
	mgr        *gp.Manager
	SendTransaction = mgr.Send
	RequestTransaction = mgr.RequestTransaction
	AddInbound = mgr.AddInbound
	AddOutbound = mgr.AddOutbound
	DropNeighbor = mgr.DropNeighbor
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
		// TODO: handle error
	}

	mgr = gp.NewManager(trans, zLogger, getTransaction)
}

func configureEvents() {
	
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Debug("neighbor removed: " + ev.DroppedID.String())
		DropNeighbor(ev.DroppedID)
	}))

	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Debug("accepted neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			//address, port, _ := net.SplitHostPort(ev.Peer.Services().Get(service.GossipKey).String())
			AddInbound(ev.Peer)
		}
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Debug("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			//address, port, _ := net.SplitHostPort(ev.Peer.Services().Get(service.GossipKey).String())
			AddOutbound(ev.Peer)
		}
	}))

	// mgr.Events.NewTransaction.Attach(events.NewClosure(func(ev *gp.NewTransactionEvent) {
	// 	tx := ev.Body
	// 	metaTx := meta_transaction.FromBytes(tx)
	// 	Events.NewTransaction.Trigger(metaTx)
	// }))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *meta_transaction.MetaTransaction) {
		t := &pb.Transaction{
			Body: tx.GetBytes(),
		}
		b, err := proto.Marshal(t)
		if err != nil {
			return
		}
		SendTransaction(b)
	}))
}
