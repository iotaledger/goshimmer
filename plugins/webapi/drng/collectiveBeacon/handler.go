package collectiveBeacon

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// Handler creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
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
		return c.JSON(http.StatusBadRequest, Response{Error: "Not a valid Collective Beacon payload"})
	}

	tx := messagelayer.MessageFactory.IssuePayload(parsedPayload)

	return c.JSON(http.StatusOK, Response{Id: tx.Id().String()})
}

type Response struct {
	Id    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type Request struct {
	Payload []byte `json:"payload"`
}
