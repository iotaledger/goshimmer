// remotelog is a plugin that enables log messages being sent via UDP to a central ELK stack for debugging.
// It is disabled by default and when enabled, additionally, logger.disableEvents=false in config.json needs to be set.
// The destination can be set via logger.remotelog.serverAddress.
// All events according to logger.level in config.json are sent.
package remotelog

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

type LogMessage struct {
	NodeId    string    `json:"nodeId"`
	Level     string    `json:"level"`
	Name      string    `json:"name"`
	Msg       string    `json:"msg"`
	Timestamp time.Time `json:"timestamp"`
}

const (
	CFG_SERVER_ADDRESS = "logger.remotelog.serverAddress"
	CFG_DISABLE_EVENTS = "logger.disableEvents"
	PLUGIN_NAME        = "remotelog"
)

var (
	PLUGIN = node.NewPlugin(PLUGIN_NAME, node.Disabled, configure)
	conn   net.Conn
	myID   string
)

func configure(plugin *node.Plugin) {
	log := logger.NewLogger(PLUGIN_NAME)

	if parameter.NodeConfig.GetBool(CFG_DISABLE_EVENTS) {
		log.Warnf("%s in config.json needs to be false so that events can be captured!", CFG_DISABLE_EVENTS)
		return
	}

	c, err := net.Dial("udp", parameter.NodeConfig.GetString(CFG_SERVER_ADDRESS))
	if err != nil {
		log.Warnf("Could not create UDP socket to '%s'. %v", parameter.NodeConfig.GetString(CFG_SERVER_ADDRESS), err)
		return
	}
	conn = c

	if local.GetInstance() != nil {
		myID = hex.EncodeToString(local.GetInstance().ID().Bytes())
	}

	logger.Events.AnyMsg.Attach(events.NewClosure(sendLogMsg))
}

func sendLogMsg(level logger.Level, name string, msg string) {
	go func() {
		m := LogMessage{myID, level.CapitalString(), name, msg, time.Now()}
		b, _ := json.Marshal(m)
		fmt.Fprint(conn, string(b))
	}()
}
