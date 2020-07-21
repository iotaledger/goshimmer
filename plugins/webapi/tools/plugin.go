package tools

import (
	"container/list"
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

// PluginName is the name of the web API tools endpoint plugin.
const PluginName = "WebAPI tools Endpoint"

var (
	// plugin is the plugin instance of the web API tools endpoint plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	webapi.Server().GET("tools/pastcone", pastCone)
}

func pastCone(c echo.Context) error {
	var checkedMessageCount int
	var request PastConeRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, PastConeResponse{Error: err.Error()})
	}

	log.Info("Received:", request.ID)

	msgID, err := message.NewId(request.ID)
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, PastConeResponse{Error: err.Error()})
	}

	// create a new stack that hold messages to check
	stack := list.New()
	stack.PushBack(msgID)
	// keep track of submitted checks (to not re-add something to the stack that is already in it)
	// searching in double-linked list is quite expensive, but not in a map
	submitted := make(map[message.Id]bool)

	// process messages in stack, try to request parents until we end up at the genesis
	for stack.Len() > 0 {
		checkedMessageCount++
		// pop the first element from stack
		currentMsgElement := stack.Front()
		currentMsgID := currentMsgElement.Value.(message.Id)
		stack.Remove(currentMsgElement)

		// ask node if it has it
		msgObject := messagelayer.Tangle().Message(currentMsgID)
		msgMetadataObject := messagelayer.Tangle().MessageMetadata(currentMsgID)

		if !msgObject.Exists() || !msgMetadataObject.Exists() {
			return c.JSON(http.StatusOK, PastConeResponse{Exist: false, PastConeSize: checkedMessageCount, Error: fmt.Sprintf("couldn't find %s message on node", currentMsgID)})
		}

		// get trunk and branch
		msg := msgObject.Unwrap()
		branchID := msg.BranchId()
		trunkID := msg.TrunkId()

		// release objects
		msgObject.Release()
		msgMetadataObject.Release()

		if branchID == message.EmptyId && msg.TrunkId() == message.EmptyId {
			// msg only attaches to genesis
			continue
		} else {
			if !submitted[branchID] && branchID != message.EmptyId {
				stack.PushBack(branchID)
				submitted[branchID] = true
			}
			if !submitted[trunkID] && trunkID != message.EmptyId {
				stack.PushBack(trunkID)
				submitted[trunkID] = true
			}
		}
	}
	return c.JSON(http.StatusOK, PastConeResponse{Exist: true, PastConeSize: checkedMessageCount})
}

// PastConeRequest holds the message id to query.
type PastConeRequest struct {
	ID string `json:"id"`
}

// PastConeResponse is the HTTP response containing the number of messages in the past cone and if all messages of the past cone
// exist on the node.
type PastConeResponse struct {
	Exist        bool   `json:"exist,omitempty"`
	PastConeSize int    `json:"pastConeSize,omitempty"`
	Error        string `json:"error,omitempty"`
}
