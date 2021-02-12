package drng

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

// collectiveBeaconHandler gets the current DRNG committee.
func collectiveBeaconHandler(c echo.Context) error {
	var request CollectiveBeaconRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, CollectiveBeaconResponse{Error: err.Error()})
	}

	marshalUtil := marshalutil.New(request.Payload)
	parsedPayload, err := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	if err != nil {
		return c.JSON(http.StatusBadRequest, CollectiveBeaconResponse{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(parsedPayload, messagelayer.Tangle())
	if err != nil {
		return c.JSON(http.StatusBadRequest, CollectiveBeaconResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, CollectiveBeaconResponse{ID: msg.ID().String()})
}

// CollectiveBeaconResponse is the HTTP response from broadcasting a collective beacon message.
type CollectiveBeaconResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// CollectiveBeaconRequest is a request containing a collective beacon response.
type CollectiveBeaconRequest struct {
	Payload []byte `json:"payload"`
}
