package webapi

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

func init() {
	Server().GET("tools/message/missing", missingHandler)
}

// missingHandler process missing requests.
func missingHandler(c echo.Context) error {
	if _, exists := DisabledAPIs[ToolsRoot]; exists {
		return c.JSON(http.StatusForbidden, MissingResponse{Error: "Forbidden endpoint"})
	}

	res := &MissingResponse{}
	missingIDs := messagelayer.Tangle().MissingMessages()
	for _, msg := range missingIDs {
		res.IDs = append(res.IDs, msg.String())
	}
	res.Count = len(missingIDs)
	return c.JSON(http.StatusOK, res)
}

// MissingResponse is the HTTP response containing all the missing messages and their count.
type MissingResponse struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
	Error string   `json:"error,omitempty"`
}
