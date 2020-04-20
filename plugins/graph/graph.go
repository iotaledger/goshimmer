package graph

import (
	"container/ring"
	"fmt"
	"strings"

	socketio "github.com/googollee/go-socket.io"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/iota.go/consts"

	"github.com/iotaledger/goshimmer/plugins/config"

	"github.com/iotaledger/hive.go/syncutils"
)

const (
	MessageBufferSize = 1800
)

var (
	msgRingBuffer *ring.Ring // messages
	snRingBuffer  *ring.Ring // confirmed messages
	msRingBuffer  *ring.Ring // Milestones

	broadcastLock     = syncutils.Mutex{}
	msgRingBufferLock = syncutils.Mutex{}
)

type wsMessage struct {
	Hash            string `json:"hash"`
	Address         string `json:"address"`
	Value           string `json:"value"`
	Tag             string `json:"tag"`
	Timestamp       string `json:"timestamp"`
	CurrentIndex    string `json:"current_index"`
	LastIndex       string `json:"last_index"`
	Bundle          string `json:"bundle_hash"`
	TrunkMessageId  string `json:"transaction_trunk"`
	BranchMessageId string `json:"transaction_branch"`
}

type wsMessageSn struct {
	Hash            string `json:"hash"`
	Address         string `json:"address"`
	TrunkMessageId  string `json:"transaction_trunk"`
	BranchMessageId string `json:"transaction_branch"`
	Bundle          string `json:"bundle"`
}

type wsConfig struct {
	NetworkName string `json:"networkName"`
}

func initRingBuffers() {
	msgRingBuffer = ring.New(MessageBufferSize)
	snRingBuffer = ring.New(MessageBufferSize)
	msRingBuffer = ring.New(20)
}

func onConnectHandler(s socketio.Conn) error {
	infoMsg := "Graph client connection established"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(infoMsg)
	socketioServer.JoinRoom("broadcast", s)

	config := &wsConfig{NetworkName: config.Node.GetString(CFG_NETWORK)}

	var initMsgs []*wsMessage
	msgRingBuffer.Do(func(wsMsg interface{}) {
		if wsMsg != nil {
			initMsgs = append(initMsgs, wsMsg.(*wsMessage))
		}
	})

	var initSns []*wsMessageSn
	snRingBuffer.Do(func(sn interface{}) {
		if sn != nil {
			initSns = append(initSns, sn.(*wsMessageSn))
		}
	})

	var initMs []string
	msRingBuffer.Do(func(ms interface{}) {
		if ms != nil {
			initMs = append(initMs, ms.(string))
		}
	})

	s.Emit("config", config)
	s.Emit("inittx", initMsgs) // needs to be 'tx' for now
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

func onAttachedMessage(msg *message.Message) {
	wsMsg := &wsMessage{
		Hash:            msg.Id().String(),
		Address:         "",
		Value:           "0",
		Tag:             emptyTag,
		Timestamp:       "0",
		CurrentIndex:    "0",
		LastIndex:       "0",
		Bundle:          consts.NullHashTrytes,
		TrunkMessageId:  msg.TrunkId().String(),
		BranchMessageId: msg.BranchId().String(),
	}

	msgRingBufferLock.Lock()
	msgRingBuffer.Value = wsMsg
	msgRingBuffer = msgRingBuffer.Next()
	msgRingBufferLock.Unlock()

	broadcastLock.Lock()
	// needs to use  'tx' for now
	socketioServer.BroadcastToRoom("broadcast", "tx", wsMsg)
	broadcastLock.Unlock()
}
