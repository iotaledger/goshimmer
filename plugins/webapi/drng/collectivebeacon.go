package drng

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// collectiveBeaconHandler gets the current DRNG committee.
func collectiveBeaconHandler(c echo.Context) error {
	var request jsonmodels2.CollectiveBeaconRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels2.CollectiveBeaconResponse{Error: err.Error()})
	}

	marshalUtil := marshalutil.New(request.Payload)
	parsedPayload, err := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.CollectiveBeaconResponse{Error: err.Error()})
	}

	msg, err := messagelayer.Tangle().IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels2.CollectiveBeaconResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, jsonmodels2.CollectiveBeaconResponse{ID: msg.ID().Base58()})
}
