package message

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// ApprovalHandler runs the approval analysis.
func ApprovalHandler(c echo.Context) error {
	res := &ApprovalResponse{}
	res.Err = messagelayer.Tangle().FirstApprovalAnalysis(local.GetInstance().Identity.ID().String(), "")
	if res.Err != nil {
		c.JSON(http.StatusInternalServerError, res)
	}
	return c.JSON(http.StatusOK, res)
}

// ApprovalResponse is the HTTP response.
type ApprovalResponse struct {
	Err error `json:"error,omitempty"`
}
