package message

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// MissingHandler process missing requests.
func MissingHandler(c echo.Context) error {
	res := &MissingResponse{}
	missingIDs := messagelayer.Tangle().Storage.MissingMessages()
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
}
