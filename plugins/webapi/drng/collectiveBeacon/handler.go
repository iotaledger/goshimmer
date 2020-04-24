package collectivebeacon

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler gets the current DRNG committee.
func Handler(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	//TODO: to check max payload size allowed, if exceeding return an error
	marshalUtil := marshalutil.New(request.Payload)
	parsedPayload, err := payload.Parse(marshalUtil)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: "not a valid Collective Beacon payload"})
	}

	msg := messagelayer.MessageFactory.IssuePayload(parsedPayload)
	return c.JSON(http.StatusOK, Response{ID: msg.Id().String()})
}

// Response is the HTTP response from broadcasting a collective beacon message.
type Response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// Request is a request containing a collective beacon response.
type Request struct {
	Payload []byte `json:"payload"`
}
