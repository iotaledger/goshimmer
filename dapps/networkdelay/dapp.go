package networkdelay

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	messageTangle "github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "NetworkDelay"

	remoteLogType = "networkdelay"
)

var (
	// App is the "plugin" instance of the network delay application.
	App = node.NewPlugin(PluginName, node.Disabled, configure)

	// log holds a reference to the logger used by this app.
	log *logger.Logger

	remoteLogConnection net.Conn
	myID                string
)

func configure(_ *node.Plugin) {
	// configure logger
	log = logger.NewLogger(PluginName)

	remoteLogConnection = remotelog.Connection()

	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
	}

	configureWebAPI()

	// subscribe to message-layer
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))
}

func onReceiveMessageFromMessageLayer(cachedMessage *message.CachedMessage, cachedMessageMetadata *messageTangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	solidMessage := cachedMessage.Unwrap()
	if solidMessage == nil {
		log.Debug("failed to unpack solid message from message layer")

		return
	}

	messagePayload := solidMessage.Payload()
	if messagePayload.Type() != Type {
		return
	}

	networkDelayObject, ok := messagePayload.(*Object)
	if !ok {
		log.Info("could not cast payload to network delay object")

		return
	}

	now := time.Now().UnixNano()

	// abort if message was sent more than 1min ago
	// this should only happen due to a node resyncing
	fmt.Println(time.Duration(now-networkDelayObject.sentTime), time.Minute)
	if time.Duration(now-networkDelayObject.sentTime) > time.Minute {
		log.Debugf("Received network delay message with >1min delay\n%s", networkDelayObject)
		return
	}

	sendToRemoteLog(networkDelayObject, now)
}

func sendToRemoteLog(networkDelayObject *Object, receiveTime int64) {
	m := networkDelay{
		NodeID:      myID,
		ID:          networkDelayObject.id.String(),
		SentTime:    networkDelayObject.sentTime,
		ReceiveTime: receiveTime,
		Delta:       receiveTime - networkDelayObject.sentTime,
		Type:        remoteLogType,
	}
	b, _ := json.Marshal(m)
	fmt.Fprint(remoteLogConnection, string(b))
	fmt.Println(string(b))
}

type networkDelay struct {
	NodeID      string `json:"nodeId"`
	ID          string `json:"id"`
	SentTime    int64  `json:"sentTime"`
	ReceiveTime int64  `json:"receiveTime"`
	Delta       int64  `json:"delta"`
	Type        string `json:"type"`
}
