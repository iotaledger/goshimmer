package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
)

func init() {
	Server().POST("drng/collectiveBeacon", collectiveBeaconHandler)
}

// collectiveBeaconHandler gets the current DRNG committee.
func collectiveBeaconHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[DrngRoot]; exists {
		return c.JSON(http.StatusForbidden, CollectiveBeaconResponse{Error: "Forbidden endpoint"})
	}

	var request CollectiveBeaconRequest
	if err := c.Bind(&request); err != nil {
		Log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, CollectiveBeaconResponse{Error: err.Error()})
	}

	marshalUtil := marshalutil.New(request.Payload)
	parsedPayload, err := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	if err != nil {
		return c.JSON(http.StatusBadRequest, CollectiveBeaconResponse{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(parsedPayload)
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
