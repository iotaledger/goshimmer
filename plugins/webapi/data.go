package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/labstack/echo"
)

const Data = "data"

func init() {
	Server().POST("data", broadcastData)
}

// broadcastData creates a message of the given payload and
// broadcasts it to the node's neighbors. It returns the message ID if successful.
func broadcastData(c echo.Context) error {
	if _, exists := DisabledAPIs[Data]; exists {
		return c.JSON(http.StatusForbidden, DataResponse{Error: "Forbidden endpoint"})
	}

	var request DataRequest
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, DataResponse{Error: err.Error()})
	}

	msg, err := issuer.IssuePayload(tangle.NewDataPayload(request.Data))
	if err != nil {
		return c.JSON(http.StatusBadRequest, DataResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, DataResponse{ID: msg.ID().String()})
}

// DataResponse contains the ID of the message sent.
type DataResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// DataRequest contains the data of the message to send.
type DataRequest struct {
	Data []byte `json:"data"`
}
