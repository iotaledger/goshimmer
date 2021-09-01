package message

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("WebAPIMessageEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("messages/:messageID", GetMessage)
	deps.Server.GET("messages/:messageID/metadata", GetMessageMetadata)
	deps.Server.GET("messages/:messageID/consensus", GetMessageConsensusMetadata)
	deps.Server.POST("messages/payload", PostPayload)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetMessage ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetMessage is the handler for the /messages/:messageID endpoint.
func GetMessage(c echo.Context) (err error) {
	messageID, err := messageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		err = c.JSON(http.StatusOK, jsonmodels.Message{
			ID:              message.ID().Base58(),
			StrongParents:   message.StrongParents().ToStrings(),
			WeakParents:     message.WeakParents().ToStrings(),
			StrongApprovers: deps.Tangle.Utils.ApprovingMessageIDs(message.ID(), tangle.StrongApprover).ToStrings(),
			WeakApprovers:   deps.Tangle.Utils.ApprovingMessageIDs(message.ID(), tangle.WeakApprover).ToStrings(),
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

	if deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		err = c.JSON(http.StatusOK, NewMessageMetadata(messageMetadata))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load MessageMetadata with %s", messageID)))
}

// NewMessageMetadata returns MessageMetadata from the given tangle.MessageMetadata.
func NewMessageMetadata(metadata *tangle.MessageMetadata) jsonmodels.MessageMetadata {
	branchID, err := deps.Tangle.Booker.MessageBranchID(metadata.ID())
	if err != nil {
		branchID = ledgerstate.BranchID{}
	}

	return jsonmodels.MessageMetadata{
		ID:                 metadata.ID().Base58(),
		ReceivedTime:       metadata.ReceivedTime().Unix(),
		Solid:              metadata.IsSolid(),
		SolidificationTime: metadata.SolidificationTime().Unix(),
		StructureDetails:   jsonmodels.NewStructureDetails(metadata.StructureDetails()),
		BranchID:           branchID.String(),
		Scheduled:          metadata.Scheduled(),
		ScheduledTime:      metadata.ScheduledTime().Unix(),
		ScheduledBypass:    metadata.ScheduledBypass(),
		Booked:             metadata.IsBooked(),
		BookedTime:         metadata.BookedTime().Unix(),
		Eligible:           metadata.IsEligible(),
		Invalid:            metadata.IsInvalid(),
		Finalized:          metadata.IsFinalized(),
		FinalizedTime:      metadata.FinalizedTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetMessageMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// GetMessageConsensusMetadata is the handler for the /messages/:messageID/consensus endpoint.
func GetMessageConsensusMetadata(c echo.Context) (err error) {
	messageID, err := messageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	consensusMechanism := deps.Tangle.Options.ConsensusMechanism.(*fcob.ConsensusMechanism)
	if consensusMechanism != nil {
		if consensusMechanism.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *fcob.MessageMetadata) {
			consensusMechanism.Storage.TimestampOpinion(messageID).Consume(func(timestampOpinion *fcob.TimestampOpinion) {
				err = c.JSON(http.StatusOK, jsonmodels.NewMessageConsensusMetadata(messageMetadata, timestampOpinion))
			})
		}) {
			return
		}
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load MessageConsensusMetadata with %s", messageID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostPayload //////////////////////////////////////////////////////////////////////////////////////////////////

// PostPayload is the handler for the /messages/payload endpoint.
func PostPayload(c echo.Context) error {
	var request jsonmodels.PostPayloadRequest
	if err := c.Bind(&request); err != nil {
		Plugin.LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	parsedPayload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	msg, err := deps.Tangle.IssuePayload(parsedPayload)
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
