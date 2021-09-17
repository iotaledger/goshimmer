package message

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// plugin holds the singleton instance of the plugin.
	plugin *node.Plugin

	// pluginOnce is used to ensure that the plugin is a singleton.
	once sync.Once
)

// Plugin returns the plugin as a singleton.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin("WebAPI message Endpoint", node.Enabled, func(*node.Plugin) {
			webapi.Server().GET("messages/:messageID", GetMessage)
			webapi.Server().GET("messages/:messageID/metadata", GetMessageMetadata)
			webapi.Server().POST("messages/payload", PostPayload)
		})
	})

	return plugin
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetMessage ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetMessage is the handler for the /messages/:messageID endpoint.
func GetMessage(c echo.Context) (err error) {
	messageID, err := messageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		err = c.JSON(http.StatusOK, jsonmodels.Message{
			ID:              message.ID().Base58(),
			StrongParents:   message.ParentsByType(tangle.StrongParentType).ToStrings(),
			WeakParents:     message.ParentsByType(tangle.WeakParentType).ToStrings(),
			StrongApprovers: messagelayer.Tangle().Utils.ApprovingMessageIDs(message.ID(), tangle.StrongApprover).ToStrings(),
			WeakApprovers:   messagelayer.Tangle().Utils.ApprovingMessageIDs(message.ID(), tangle.WeakApprover).ToStrings(),
			IssuerPublicKey: message.IssuerPublicKey().String(),
			IssuingTime:     message.IssuingTime().Unix(),
			SequenceNumber:  message.SequenceNumber(),
			PayloadType:     message.Payload().Type().String(),
			TransactionID: func() string {
				if message.Payload().Type() == ledgerstate.TransactionType {
					return message.Payload().(*ledgerstate.Transaction).ID().Base58()
				}

				return ""
			}(),
			Payload:   message.Payload().Bytes(),
			Signature: message.Signature().String(),
		})
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Message with %s", messageID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetMessageMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// GetMessageMetadata is the handler for the /messages/:messageID/metadata endpoint.
func GetMessageMetadata(c echo.Context) (err error) {
	messageID, err := messageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		fmt.Println(messageMetadata)
		err = c.JSON(http.StatusOK, NewMessageMetadata(messageMetadata))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load MessageMetadata with %s", messageID)))
}

// NewMessageMetadata returns MessageMetadata from the given tangle.MessageMetadata.
func NewMessageMetadata(metadata *tangle.MessageMetadata) jsonmodels.MessageMetadata {
	branchID, err := messagelayer.Tangle().Booker.MessageBranchID(metadata.ID())
	if err != nil {
		branchID = ledgerstate.BranchID{}
	}

	return jsonmodels.MessageMetadata{
		ID:                  metadata.ID().Base58(),
		ReceivedTime:        metadata.ReceivedTime().Unix(),
		Solid:               metadata.IsSolid(),
		SolidificationTime:  metadata.SolidificationTime().Unix(),
		StructureDetails:    jsonmodels.NewStructureDetails(metadata.StructureDetails()),
		BranchID:            branchID.String(),
		Scheduled:           metadata.Scheduled(),
		ScheduledTime:       metadata.ScheduledTime().Unix(),
		ScheduledBypass:     metadata.ScheduledBypass(),
		Booked:              metadata.IsBooked(),
		BookedTime:          metadata.BookedTime().Unix(),
		Invalid:             metadata.IsInvalid(),
		GradeOfFinality:     metadata.GradeOfFinality(),
		GradeOfFinalityTime: metadata.GradeOfFinalityTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostPayload //////////////////////////////////////////////////////////////////////////////////////////////////

// PostPayload is the handler for the /messages/payload endpoint.
func PostPayload(c echo.Context) error {
	var request jsonmodels.PostPayloadRequest
	if err := c.Bind(&request); err != nil {
		Plugin().LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	parsedPayload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	msg, err := messagelayer.Tangle().IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewPostPayloadResponse(msg))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region messageIDFromContext /////////////////////////////////////////////////////////////////////////////////////////

// messageIDFromContext determines the MessageID from the messageID parameter in an echo.Context. It expects it to
// either be a base58 encoded string or the builtin alias EmptyMessageID.
func messageIDFromContext(c echo.Context) (messageID tangle.MessageID, err error) {
	switch messageIDString := c.Param("messageID"); messageIDString {
	case "EmptyMessageID":
		messageID = tangle.EmptyMessageID
	default:
		messageID, err = tangle.NewMessageID(messageIDString)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
