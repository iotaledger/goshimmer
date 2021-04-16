package message

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
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
			webapi.Server().POST("messages/payload/:delay", PostPayload)
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
		err = c.JSON(http.StatusOK, jsonmodels.NewMessage(message))
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
		err = c.JSON(http.StatusOK, jsonmodels.NewMessageMetadata(messageMetadata))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load MessageMetadata with %s", messageID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostPayload //////////////////////////////////////////////////////////////////////////////////////////////////

// PostPayload is the handler for the /messages/payload endpoint.
func PostPayload(c echo.Context) error {

	delay, err := delayFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	var request jsonmodels.PostPayloadRequest
	if err := c.Bind(&request); err != nil {
		Plugin().LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	parsedPayload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if delay == 0 {
		msg, err := messagelayer.Tangle().IssuePayload(parsedPayload)
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
		}
		return c.JSON(http.StatusOK, jsonmodels.NewPostPayloadResponse(msg))
	}
	msg, err := messagelayer.Tangle().IssuePayloadWithDelay(parsedPayload, delay)
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

func delayFromContext(c echo.Context) (delay time.Duration, err error) {
	delayString, err := strconv.Atoi(c.Param("delay"))
	if err != nil {
		return delay, err
	}
	delay = time.Duration(delayString)
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
