package pastcone

import (
	"container/list"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// Handler process a pastcone request.
func Handler(c echo.Context) error {
	var checkedMessageCount int
	var request Request
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	msgID, err := message.NewID(request.ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
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
			return c.JSON(http.StatusOK, Response{Exist: false, PastConeSize: checkedMessageCount, Error: fmt.Sprintf("couldn't find %s message on node", currentMsgID)})
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
	return c.JSON(http.StatusOK, Response{Exist: true, PastConeSize: checkedMessageCount})
}

// Request holds the message id to query.
type Request struct {
	ID string `json:"id"`
}

// Response is the HTTP response containing the number of messages in the past cone and if all messages of the past cone
// exist on the node.
type Response struct {
	Exist        bool   `json:"exist,omitempty"`
	PastConeSize int    `json:"pastConeSize,omitempty"`
	Error        string `json:"error,omitempty"`
}
