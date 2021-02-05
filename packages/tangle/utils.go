package tangle

import (
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/types"
)

// region Utils ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Utils is a Tangle component that bundles methods that can be used to interact with the Tangle, that do not belong
// into public API.
type Utils struct {
	tangle *Tangle
}

// NewUtils is the constructor of the Utils component.
func NewUtils(tangle *Tangle) (utils *Utils) {
	return &Utils{
		tangle: tangle,
	}
}

// region walkers //////////////////////////////////////////////////////////////////////////////////////////////////////

// WalkMessageID is a generic Tangle walker that executes a custom callback for every visited MessageID, starting from
// the given entry points. It accepts an optional boolean parameter which can be set to true if a Message should be
// visited more than once following different paths. The callback receives a Walker object as the last parameter which
// can be used to control the behavior of the walk similar to how a "Context" is used in some parts of the stdlib.
func (u *Utils) WalkMessageID(callback func(messageID MessageID, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	if len(entryPoints) == 0 {
		panic("you need to provide at least one entry point")
	}

	messageIDWalker := walker.New(revisitElements...)
	for _, messageID := range entryPoints {
		messageIDWalker.Push(messageID)
	}

	for messageIDWalker.HasNext() {
		callback(messageIDWalker.Next().(MessageID), messageIDWalker)
	}
}

// WalkMessage is a generic Tangle walker that executes a custom callback for every visited Message, starting from
// the given entry points. It accepts an optional boolean parameter which can be set to true if a Message should be
// visited more than once following different paths. The callback receives a Walker object as the last parameter which
// can be used to control the behavior of the walk similar to how a "Context" is used in some parts of the stdlib.
func (u *Utils) WalkMessage(callback func(message *Message, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			callback(message, walker)
		})
	}, entryPoints, revisitElements...)
}

// WalkMessageMetadata is a generic Tangle walker that executes a custom callback for every visited MessageMetadata,
// starting from the given entry points. It accepts an optional boolean parameter which can be set to true if a Message
// should be visited more than once following different paths. The callback receives a Walker object as the last
// parameter which can be used to control the behavior of the walk similar to how a "Context" is used in some parts of
// the stdlib.
func (u *Utils) WalkMessageMetadata(callback func(messageMetadata *MessageMetadata, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
		u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			callback(messageMetadata, walker)
		})
	}, entryPoints, revisitElements...)
}

// WalkMessageAndMetadata is a generic Tangle walker that executes a custom callback for every visited Message and
// MessageMetadata, starting from the given entry points. It accepts an optional boolean parameter which can be set to
// true if a Message should be visited more than once following different paths. The callback receives a Walker object
// as the last parameter which can be used to control the behavior of the walk similar to how a "Context" is used in
// some parts of the stdlib.
func (u *Utils) WalkMessageAndMetadata(callback func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				callback(message, messageMetadata, walker)
			})
		})
	}, entryPoints, revisitElements...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region structural checks ////////////////////////////////////////////////////////////////////////////////////////////

func (u *Utils) IsInPastCone(earlierMessageID MessageID, laterMessageID MessageID) bool {
	if u.IsInStrongPastCone(earlierMessageID, laterMessageID) {
		return true
	}

	cachedWeakApprovers := u.tangle.Storage.Approvers(earlierMessageID, WeakApprover)
	defer cachedWeakApprovers.Release()

	for _, weakApprover := range cachedWeakApprovers.Unwrap() {
		if weakApprover == nil {
			continue
		}

		if u.IsInStrongPastCone(weakApprover.ApproverMessageID(), laterMessageID) {
			return true
		}
	}

	return false
}

func (u *Utils) IsInStrongPastCone(earlierMessageID MessageID, laterMessageID MessageID) (isInStrongPastCone bool) {
	// return early if the MessageIDs are the same
	if earlierMessageID == laterMessageID {
		return true
	}

	// retrieve the StructureDetails of both messages
	var earlierMessageStructureDetails, laterMessageStructureDetails *markers.StructureDetails
	u.tangle.Storage.MessageMetadata(earlierMessageID).Consume(func(messageMetadata *MessageMetadata) {
		earlierMessageStructureDetails = messageMetadata.StructureDetails()
	})
	u.tangle.Storage.MessageMetadata(laterMessageID).Consume(func(messageMetadata *MessageMetadata) {
		laterMessageStructureDetails = messageMetadata.StructureDetails()
	})

	// return false if one of the StructureDetails could not be retrieved (one of the messages was removed already / not
	// booked yet)
	if earlierMessageStructureDetails == nil || laterMessageStructureDetails == nil {
		return false
	}

	// perform past cone check
	switch u.tangle.MarkersManager.IsInPastCone(earlierMessageStructureDetails, laterMessageStructureDetails) {
	case types.True:
		isInStrongPastCone = true
	case types.False:
		isInStrongPastCone = false
	case types.Maybe:
		u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
			if messageID == laterMessageID {
				isInStrongPastCone = true
				walker.StopWalk()
				return
			}

			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				if structureDetails := messageMetadata.StructureDetails(); structureDetails != nil && !structureDetails.IsPastMarker {
					for _, approvingMessageID := range u.tangle.Utils.ApprovingMessageIDs(messageID, StrongApprover) {
						walker.Push(approvingMessageID)
					}
				}
			})
		}, u.ApprovingMessageIDs(earlierMessageID, StrongApprover))
	}

	return
}

func (u *Utils) ApprovingMessageIDs(messageID MessageID, optionalApproverType ...ApproverType) (approvingMessageIDs MessageIDs) {
	approvingMessageIDs = make(MessageIDs, 0)
	u.tangle.Storage.Approvers(messageID, optionalApproverType...).Consume(func(approver *Approver) {
		approvingMessageIDs = append(approvingMessageIDs, approver.ApproverMessageID())
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
