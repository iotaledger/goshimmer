package graph

import (
	"container/ring"
	"fmt"
	"strconv"
	"strings"

	socketio "github.com/googollee/go-socket.io"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/iota.go/consts"

	"github.com/iotaledger/hive.go/syncutils"
)

const (
	TX_BUFFER_SIZE = 1800
)

var (
	txRingBuffer *ring.Ring // transactions
	snRingBuffer *ring.Ring // confirmed transactions
	msRingBuffer *ring.Ring // Milestones

	broadcastLock    = syncutils.Mutex{}
	txRingBufferLock = syncutils.Mutex{}
)

type wsTransaction struct {
	Hash              string `json:"hash"`
	Address           string `json:"address"`
	Value             string `json:"value"`
	Tag               string `json:"tag"`
	Timestamp         string `json:"timestamp"`
	CurrentIndex      string `json:"current_index"`
	LastIndex         string `json:"last_index"`
	Bundle            string `json:"bundle_hash"`
	TrunkTransaction  string `json:"transaction_trunk"`
	BranchTransaction string `json:"transaction_branch"`
}

type wsTransactionSn struct {
	Hash              string `json:"hash"`
	Address           string `json:"address"`
	TrunkTransaction  string `json:"transaction_trunk"`
	BranchTransaction string `json:"transaction_branch"`
	Bundle            string `json:"bundle"`
}

type wsConfig struct {
	NetworkName string `json:"networkName"`
}

func initRingBuffers() {
	txRingBuffer = ring.New(TX_BUFFER_SIZE)
	snRingBuffer = ring.New(TX_BUFFER_SIZE)
	msRingBuffer = ring.New(20)
}

func onConnectHandler(s socketio.Conn) error {
	infoMsg := "Graph client connection established"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(infoMsg)
	socketioServer.JoinRoom("broadcast", s)

	config := &wsConfig{NetworkName: parameter.NodeConfig.GetString("graph.networkName")}

	var initTxs []*wsTransaction
	txRingBuffer.Do(func(tx interface{}) {
		if tx != nil {
			initTxs = append(initTxs, tx.(*wsTransaction))
		}
	})

	var initSns []*wsTransactionSn
	snRingBuffer.Do(func(sn interface{}) {
		if sn != nil {
			initSns = append(initSns, sn.(*wsTransactionSn))
		}
	})

	var initMs []string
	msRingBuffer.Do(func(ms interface{}) {
		if ms != nil {
			initMs = append(initMs, ms.(string))
		}
	})

	s.Emit("config", config)
	s.Emit("inittx", initTxs)
	s.Emit("initsn", initSns)
	s.Emit("initms", initMs)
	s.Emit("donation", "0")
	s.Emit("donations", []int{})
	s.Emit("donation-address", "-")

	return nil
}

func onErrorHandler(conn socketio.Conn, e error) {
	errorMsg := "Graph meet error"
	if e != nil {
		errorMsg = fmt.Sprintf("%s: %s", errorMsg, e.Error())
	}
	log.Error(errorMsg)
}

func onDisconnectHandler(s socketio.Conn, msg string) {
	infoMsg := "Graph client connection closed"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(fmt.Sprintf("%s: %s", infoMsg, msg))
	socketioServer.LeaveAllRooms(s)
}

var emptyTag = strings.Repeat("9", consts.TagTrinarySize/3)

func onNewTx(tx *value_transaction.ValueTransaction) {
	wsTx := &wsTransaction{
		Hash:              tx.GetHash(),
		Address:           tx.GetAddress(),
		Value:             strconv.FormatInt(tx.GetValue(), 10),
		Tag:               emptyTag,
		Timestamp:         strconv.FormatInt(int64(tx.GetTimestamp()), 10),
		CurrentIndex:      "0",
		LastIndex:         "0",
		Bundle:            consts.NullHashTrytes,
		TrunkTransaction:  tx.GetTrunkTransactionHash(),
		BranchTransaction: tx.GetBranchTransactionHash(),
	}

	txRingBufferLock.Lock()
	txRingBuffer.Value = wsTx
	txRingBuffer = txRingBuffer.Next()
	txRingBufferLock.Unlock()

	broadcastLock.Lock()
	socketioServer.BroadcastToRoom("broadcast", "tx", wsTx)
	broadcastLock.Unlock()
}
