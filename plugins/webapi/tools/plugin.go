package tools

import (
	"container/list"
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	valueTangle "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"

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
	webapi.Server().GET("tools/missing", missing)
	webapi.Server().GET("tools/messageapprovers", messageApprovers)
	webapi.Server().GET("tools/valueapprovers", valueApprovers)

}

func valueApprovers(c echo.Context) error {
	var request ValueApproversRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, ValueApproversResponse{Error: err.Error()})
	}
	id, err := payload.NewID(request.ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ValueApproversResponse{Error: err.Error()})
	}
	var approvers []Approver
	valuetransfers.Tangle().ForeachApprovers(id, func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *valueTangle.CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *valueTangle.CachedTransactionMetadata) {
		defer cachedPayload.Release()
		defer cachedPayloadMetadata.Release()
		defer cachedTransaction.Release()
		defer cachedTransactionMetadata.Release()

		txMetadata := cachedTransactionMetadata.Unwrap()
		approver := Approver{
			PayloadID:     cachedPayload.Unwrap().ID().String(),
			TransactionID: cachedTransaction.Unwrap().ID().String(),
			InclusionState: utils.InclusionState{
				Solid:       txMetadata.Solid(),
				Confirmed:   txMetadata.Confirmed(),
				Rejected:    txMetadata.Rejected(),
				Liked:       txMetadata.Liked(),
				Conflicting: txMetadata.Conflicting(),
				Finalized:   txMetadata.Finalized(),
				Preferred:   txMetadata.Preferred(),
			},
		}
		approvers = append(approvers, approver)
	})
	return c.JSON(http.StatusOK, ValueApproversResponse{
		Approvers: approvers,
	})
}

func messageApprovers(c echo.Context) error {
	var request MessageApproversRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, MessageApproversResponse{Error: err.Error()})
	}

	msgID, err := message.NewID(request.ID)
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, MessageApproversResponse{Error: err.Error()})
	}

	var approvers []string
	cachedApprovers := messagelayer.Tangle().Approvers(msgID)
	cachedApprovers.Consume(func(approver *tangle.Approver) {
		approvers = append(approvers, approver.ApproverMessageID().String())
	})
	return c.JSON(http.StatusOK, MessageApproversResponse{IDs: approvers})
}

func pastCone(c echo.Context) error {
	var checkedMessageCount int
	var request PastConeRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, PastConeResponse{Error: err.Error()})
	}

	log.Info("Received:", request.ID)

	msgID, err := message.NewID(request.ID)
	if err != nil {
		log.Info(err)
		return c.JSON(http.StatusBadRequest, PastConeResponse{Error: err.Error()})
	}

	// create a new stack that hold messages to check
	stack := list.New()
	stack.PushBack(msgID)
	// keep track of submitted checks (to not re-add something to the stack that is already in it)
	// searching in double-linked list is quite expensive, but not in a map
	submitted := make(map[message.ID]bool)

	// process messages in stack, try to request parents until we end up at the genesis
	for stack.Len() > 0 {
		checkedMessageCount++
		// pop the first element from stack
		currentMsgElement := stack.Front()
		currentMsgID := currentMsgElement.Value.(message.ID)
		stack.Remove(currentMsgElement)

		// ask node if it has it
		msgObject := messagelayer.Tangle().Message(currentMsgID)
		msgMetadataObject := messagelayer.Tangle().MessageMetadata(currentMsgID)

		if !msgObject.Exists() || !msgMetadataObject.Exists() {
			return c.JSON(http.StatusOK, PastConeResponse{Exist: false, PastConeSize: checkedMessageCount, Error: fmt.Sprintf("couldn't find %s message on node", currentMsgID)})
		}

		// get trunk and branch
		msg := msgObject.Unwrap()
		branchID := msg.BranchID()
		trunkID := msg.TrunkID()

		// release objects
		msgObject.Release()
		msgMetadataObject.Release()

		if branchID == message.EmptyID && msg.TrunkID() == message.EmptyID {
			// msg only attaches to genesis
			continue
		} else {
			if !submitted[branchID] && branchID != message.EmptyID {
				stack.PushBack(branchID)
				submitted[branchID] = true
			}
			if !submitted[trunkID] && trunkID != message.EmptyID {
				stack.PushBack(trunkID)
				submitted[trunkID] = true
			}
		}
	}
	return c.JSON(http.StatusOK, PastConeResponse{Exist: true, PastConeSize: checkedMessageCount})
}

// ValueApproversRequest represents value approver request
type ValueApproversRequest struct {
	ID string `json:"id"`
}

// Approver represents the approver of a value payload
type Approver struct {
	PayloadID      string               `json:"payloadID"`
	TransactionID  string               `json:"transactionID"`
	InclusionState utils.InclusionState `json:"inclusionState"`
}

// ValueApproversResponse represents the response of a ValueApproverRequest
type ValueApproversResponse struct {
	Approvers []Approver `json:"approvers"`
	Error     string     `json:"error,omitempty"`
}

// MessageApproversRequest holds the message id to get its messageApprovers
type MessageApproversRequest struct {
	ID string `json:"id"`
}

// MessageApproversResponse holds the approver message IDs of a referenced message.
type MessageApproversResponse struct {
	IDs   []string `json:"ids"`
	Error string   `json:"error,omitempty"`
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

func missing(c echo.Context) error {
	res := &MissingResponse{}
	missingIDs := messagelayer.Tangle().MissingMessages()
	for _, msg := range missingIDs {
		res.IDs = append(res.IDs, msg.String())
	}
	res.Count = len(missingIDs)
	return c.JSON(http.StatusOK, res)
}

// MissingResponse is the HTTP response containing all the missing messages and their count.
type MissingResponse struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
}
