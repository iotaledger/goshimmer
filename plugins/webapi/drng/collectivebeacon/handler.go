package collectivebeacon

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/issuer"
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

	marshalUtil := marshalutil.New(request.Payload)
	parsedPayload, err := drng.ParseCollectiveBeaconPayload(marshalUtil)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: msg.ID().String()})
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
