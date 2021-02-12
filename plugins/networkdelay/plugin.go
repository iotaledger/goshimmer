package networkdelay

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	clockPlugin "github.com/iotaledger/goshimmer/plugins/clock"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "NetworkDelay"

	// CfgNetworkDelayOriginPublicKey defines the config flag of the issuer node public key.
	CfgNetworkDelayOriginPublicKey = "networkdelay.originPublicKey"

	remoteLogType = "networkdelay"
)

var (
	// App is the "plugin" instance of the network delay application.
	app  *node.Plugin
	once sync.Once

	// log holds a reference to the logger used by this app.
	log *logger.Logger

	remoteLogger *remotelog.RemoteLoggerConn

	myID            string
	myPublicKey     ed25519.PublicKey
	originPublicKey ed25519.PublicKey

	// clockEnabled defines if the clock plugin is enabled.
	clockEnabled bool
)

func init() {
	flag.String(CfgNetworkDelayOriginPublicKey, "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd", "default issuer node public key")
}

// App gets the plugin instance.
func App() *node.Plugin {
	once.Do(func() {
		app = node.NewPlugin(PluginName, node.Disabled, configure)
	})
	return app
}

func configure(_ *node.Plugin) {
	// configure logger
	log = logger.NewLogger(PluginName)

	remoteLogger = remotelog.RemoteLogger()

	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
		myPublicKey = local.GetInstance().PublicKey()
	}

	// get origin public key from config
	bytes, err := base58.Decode(config.Node().String(CfgNetworkDelayOriginPublicKey))
	if err != nil {
		log.Fatalf("could not parse %s config entry as base58. %v", CfgNetworkDelayOriginPublicKey, err)
	}
	originPublicKey, _, err = ed25519.PublicKeyFromBytes(bytes)
	if err != nil {
		log.Fatalf("could not parse %s config entry as public key. %v", CfgNetworkDelayOriginPublicKey, err)
	}

	configureWebAPI()

	// subscribe to message-layer
	messagelayer.Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))

	clockEnabled = !node.IsSkipped(clockPlugin.Plugin())
}

func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
	messagelayer.Tangle().Storage.Message(messageID).Consume(func(solidMessage *tangle.Message) {
		messagePayload := solidMessage.Payload()
		if messagePayload.Type() != Type {
			return
		}

		// check for node identity
		issuerPubKey := solidMessage.IssuerPublicKey()
		if issuerPubKey != originPublicKey || issuerPubKey == myPublicKey {
			return
		}

		networkDelayObject, ok := messagePayload.(*Object)
		if !ok {
			log.Info("could not cast payload to network delay object")

			return
		}

		now := clock.SyncedTime().UnixNano()

		// abort if message was sent more than 1min ago
		// this should only happen due to a node resyncing
		if time.Duration(now-networkDelayObject.sentTime) > time.Minute {
			log.Debugf("Received network delay message with >1min delay\n%s", networkDelayObject)
			return
		}

		sendToRemoteLog(networkDelayObject, now)
	})
}

func sendToRemoteLog(networkDelayObject *Object, receiveTime int64) {
	m := networkDelay{
		NodeID:      myID,
		ID:          networkDelayObject.id.String(),
		SentTime:    networkDelayObject.sentTime,
		ReceiveTime: receiveTime,
		Delta:       receiveTime - networkDelayObject.sentTime,
		Clock:       clockEnabled,
		Sync:        syncbeaconfollower.Synced(),
		Type:        remoteLogType,
	}
	_ = remoteLogger.Send(m)
}

func sendPoWInfo(object *Object, powDelta time.Duration) {
	m := networkDelay{
		NodeID:      myID,
		ID:          object.id.String(),
		SentTime:    0,
		ReceiveTime: 0,
		Delta:       powDelta.Nanoseconds(),
		Clock:       clockEnabled,
		Sync:        syncbeaconfollower.Synced(),
		Type:        remoteLogType,
	}
	_ = remoteLogger.Send(m)
}

type networkDelay struct {
	NodeID      string `json:"nodeId"`
	ID          string `json:"id"`
	SentTime    int64  `json:"sentTime"`
	ReceiveTime int64  `json:"receiveTime"`
	Delta       int64  `json:"delta"`
	Clock       bool   `json:"clock"`
	Sync        bool   `json:"sync"`
	Type        string `json:"type"`
}
