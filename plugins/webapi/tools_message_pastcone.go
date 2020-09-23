package webapi

import (
	"container/list"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

func init() {
	Server().GET("tools/message/pastcone", pastconeHandler)
}

// pastconeHandler process a pastcone request.
func pastconeHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[ToolsRoot]; exists {
		return c.JSON(http.StatusForbidden, PastconeResponse{Error: "Forbidden endpoint"})
	}

	var checkedMessageCount int
	var request PastconeRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, PastconeResponse{Error: err.Error()})
	}

	msgID, err := tangle.NewMessageID(request.ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, PastconeResponse{Error: err.Error()})
	}

	// create a new stack that hold messages to check
	stack := list.New()
	stack.PushBack(msgID)
	// keep track of submitted checks (to not re-add something to the stack that is already in it)
	// searching in double-linked list is quite expensive, but not in a map
	submitted := make(map[tangle.MessageID]bool)

	// process messages in stack, try to request parents until we end up at the genesis
	for stack.Len() > 0 {
		checkedMessageCount++
		// pop the first element from stack
		currentMsgElement := stack.Front()
		currentMsgID := currentMsgElement.Value.(tangle.MessageID)
		stack.Remove(currentMsgElement)

		// ask node if it has it
		msgObject := messagelayer.Tangle().Message(currentMsgID)
		msgMetadataObject := messagelayer.Tangle().MessageMetadata(currentMsgID)

		if !msgObject.Exists() || !msgMetadataObject.Exists() {
			return c.JSON(http.StatusOK, PastconeResponse{Exist: false, PastConeSize: checkedMessageCount, Error: fmt.Sprintf("couldn't find %s message on node", currentMsgID)})
		}

		// get parent1 and parent2
		msg := msgObject.Unwrap()
		parent2ID := msg.Parent2ID()
		parent1ID := msg.Parent1ID()

		// release objects
		msgObject.Release()
		msgMetadataObject.Release()

		if parent2ID == tangle.EmptyMessageID && msg.Parent1ID() == tangle.EmptyMessageID {
			// msg only attaches to genesis
			continue
		} else {
			if !submitted[parent2ID] && parent2ID != tangle.EmptyMessageID {
				stack.PushBack(parent2ID)
				submitted[parent2ID] = true
			}
			if !submitted[parent1ID] && parent1ID != tangle.EmptyMessageID {
				stack.PushBack(parent1ID)
				submitted[parent1ID] = true
			}
		}
	}
	return c.JSON(http.StatusOK, PastconeResponse{Exist: true, PastConeSize: checkedMessageCount})
}

// PastconeRequest holds the message id to query.
type PastconeRequest struct {
	ID string `json:"id"`
}

// PastconeResponse is the HTTP response containing the number of messages in the past cone and if all messages of the past cone
// exist on the node.
type PastconeResponse struct {
	Exist        bool   `json:"exist,omitempty"`
	PastConeSize int    `json:"pastConeSize,omitempty"`
	Error        string `json:"error,omitempty"`
}
