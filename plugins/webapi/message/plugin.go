package message

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
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
	deps.Server.POST("messages/payload", PostPayload)

	deps.Server.GET("messages/sequences/:sequenceID", GetSequence)
	deps.Server.GET("messages/sequences/:sequenceID/markerindexbranchidmapping", GetMarkerIndexBranchIDMapping)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetSequence //////////////////////////////////////////////////////////////////////////////////////////////////

// GetSequence is the handler for the /messages/sequences/:sequenceID endpoint.
func GetSequence(c echo.Context) (err error) {
	sequenceID, err := sequenceIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Booker.MarkersManager.Sequence(sequenceID).Consume(func(sequence *markers.Sequence) {
		messageWithLastMarker := deps.Tangle.Booker.MarkersManager.MessageID(markers.NewMarker(sequenceID, sequence.HighestIndex()))
		err = c.String(http.StatusOK, stringify.Struct("Sequence",
			stringify.StructField("ID", sequence.ID()),
			stringify.StructField("LowestIndex", sequence.LowestIndex()),
			stringify.StructField("HighestIndex", sequence.HighestIndex()),
			stringify.StructField("MessageWithLastMarker", messageWithLastMarker),
		))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Sequence with %s", sequenceID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetMarkerIndexBranchIDMapping ////////////////////////////////////////////////////////////////////////////////

// GetMarkerIndexBranchIDMapping is the handler for the /messages/sequences/:sequenceID/markerindexbranchidmapping endpoint.
func GetMarkerIndexBranchIDMapping(c echo.Context) (err error) {
	sequenceID, err := sequenceIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Storage.MarkerIndexBranchIDMapping(sequenceID).Consume(func(markerIndexBranchIDMapping *tangle.MarkerIndexBranchIDMapping) {
		err = c.String(http.StatusOK, markerIndexBranchIDMapping.String())
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load MarkerIndexBranchIDMapping of %s", sequenceID)))
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
			ID:                      message.ID().Base58(),
			StrongParents:           message.ParentsByType(tangle.StrongParentType).Base58(),
			WeakParents:             message.ParentsByType(tangle.WeakParentType).Base58(),
			ShallowLikeParents:      message.ParentsByType(tangle.ShallowLikeParentType).Base58(),
			ShallowDislikeParents:   message.ParentsByType(tangle.ShallowDislikeParentType).Base58(),
			StrongApprovers:         deps.Tangle.Utils.ApprovingMessageIDs(message.ID(), tangle.StrongApprover).Base58(),
			WeakApprovers:           deps.Tangle.Utils.ApprovingMessageIDs(message.ID(), tangle.WeakApprover).Base58(),
			ShallowLikeApprovers:    deps.Tangle.Utils.ApprovingMessageIDs(message.ID(), tangle.ShallowLikeApprover).Base58(),
			ShallowDislikeApprovers: deps.Tangle.Utils.ApprovingMessageIDs(message.ID(), tangle.ShallowDislikeApprover).Base58(),
			IssuerPublicKey:         message.IssuerPublicKey().String(),
			IssuingTime:             message.IssuingTime().Unix(),
			SequenceNumber:          message.SequenceNumber(),
			PayloadType:             message.Payload().Type().String(),
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
	branchIDs, _ := deps.Tangle.Booker.MessageBranchIDs(metadata.ID())

	return jsonmodels.MessageMetadata{
		ID:                  metadata.ID().Base58(),
		ReceivedTime:        metadata.ReceivedTime().Unix(),
		Solid:               metadata.IsSolid(),
		SolidificationTime:  metadata.SolidificationTime().Unix(),
		StructureDetails:    jsonmodels.NewStructureDetails(metadata.StructureDetails()),
		BranchIDs:           branchIDs.Base58(),
		AddedBranchIDs:      metadata.AddedBranchIDs().Base58(),
		SubtractedBranchIDs: metadata.SubtractedBranchIDs().Base58(),
		Scheduled:           metadata.Scheduled(),
		ScheduledTime:       metadata.ScheduledTime().Unix(),
		Booked:              metadata.IsBooked(),
		BookedTime:          metadata.BookedTime().Unix(),
		ObjectivelyInvalid:  metadata.IsObjectivelyInvalid(),
		SubjectivelyInvalid: metadata.IsSubjectivelyInvalid(),
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

// sequenceIDFromContext determines the sequenceID from the sequenceID parameter in an echo.Context.
func sequenceIDFromContext(c echo.Context) (id markers.SequenceID, err error) {
	sequenceIDInt, err := strconv.Atoi(c.Param("sequenceID"))
	if err != nil {
		return
	}

	return markers.SequenceID(sequenceIDInt), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
