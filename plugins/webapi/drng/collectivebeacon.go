package drng

import (
	"net/http"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

// collectiveBeaconHandler gets the current DRNG committee.
func collectiveBeaconHandler(c echo.Context) error {
	var request jsonmodels.CollectiveBeaconRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.CollectiveBeaconResponse{Error: err.Error()})
	}

	marshalUtil := marshalutil.New(request.Payload)
	parsedPayload, err := drng.CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.CollectiveBeaconResponse{Error: err.Error()})
	}

	msg, err := deps.Tangle.IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.CollectiveBeaconResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, jsonmodels.CollectiveBeaconResponse{ID: msg.ID().Base58()})
}
