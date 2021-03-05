package message

import (
	"container/list"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// PastconeHandler process a pastcone request.
func PastconeHandler(c echo.Context) error {
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
		msgObject := messagelayer.Tangle().Storage.Message(currentMsgID)
		msgMetadataObject := messagelayer.Tangle().Storage.MessageMetadata(currentMsgID)

		if !msgObject.Exists() || !msgMetadataObject.Exists() {
			return c.JSON(http.StatusOK, PastconeResponse{Exist: false, PastConeSize: checkedMessageCount, Error: fmt.Sprintf("couldn't find %s message on node", currentMsgID)})
		}

		// get parent1 and parent2
		msg := msgObject.Unwrap()

		onlyGenesis := true
		msg.ForEachParent(func(parent tangle.Parent) {
			onlyGenesis = onlyGenesis && (parent.ID == tangle.EmptyMessageID)
		})

		if onlyGenesis {
			// release objects
			msgObject.Release()
			msgMetadataObject.Release()
			// msg only attaches to genesis
			continue
		} else {
			msg.ForEachParent(func(parent tangle.Parent) {
				if !submitted[parent.ID] && parent.ID != tangle.EmptyMessageID {
					stack.PushBack(parent.ID)
					submitted[parent.ID] = true
				}
			})
		}

		// release objects
		msgObject.Release()
		msgMetadataObject.Release()
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
