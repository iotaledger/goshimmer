package client

import (
	"container/list"
	"errors"
	"fmt"
)

// genesisID is the ID of the genesis message
var genesisID = "1111111111111111111111111111111111111111111111111111111111111111"

// PastConeExist checks that all of the messages in the the past cone of a message are existing on the node
// down to the genesis. Returns the number of messages in the past cone as well.
func (api *GoShimmerAPI) PastConeExist(messageID string) (bool, int, error) {
	// create a new stack that hold messages to check
	stack := list.New()
	stack.PushBack(messageID)
	// keep track of submitted checks (to not re-add something to the stack that is already in it)
	// searching in double-linked list is quite expensive, but not in a map
	submitted := make(map[string]bool)

	var checkedMessageCount int

	// process messages, try to request parents until we end up at the genesis
	for stack.Len() > 0 {
		checkedMessageCount++
		// pop the first element from stack
		currentMsgElement := stack.Front()
		currentMsgID := currentMsgElement.Value.(string)
		stack.Remove(currentMsgElement)

		// ask node if it has it
		response, err := api.FindMessageByID([]string{currentMsgID})
		if err != nil {
			return false, checkedMessageCount, errors.New(fmt.Sprintf("error while requesting %s message: %s", currentMsgID, err.Error()))
		}

		currentMsg := response.Messages[0]
		// the first element in response.Messages is Messages{}, so its ID is empty if message doesn't exist on node
		if currentMsg.ID == "" {
			return false, checkedMessageCount, errors.New(fmt.Sprintf("message %s doesn't exist on node", currentMsgID))
		}

		// is it the genesis?
		if currentMsg.BranchID == genesisID && currentMsg.TrunkID == genesisID {
			continue
		} else {
			if !submitted[currentMsg.BranchID] {
				stack.PushBack(currentMsg.BranchID)
				submitted[currentMsg.BranchID] = true
			}
			if !submitted[currentMsg.TrunkID] {
				stack.PushBack(currentMsg.TrunkID)
				submitted[currentMsg.TrunkID] = true
			}
		}
	}
	return true, checkedMessageCount, nil
}
