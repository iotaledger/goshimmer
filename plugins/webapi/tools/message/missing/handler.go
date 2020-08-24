package missing

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
)

// Handler process missing requests.
func Handler(c echo.Context) error {
	res := &Response{}
	missingIDs := messagelayer.Tangle().MissingMessages()
	for _, msg := range missingIDs {
		res.IDs = append(res.IDs, msg.String())
	}
	res.Count = len(missingIDs)
	return c.JSON(http.StatusOK, res)
}

// Response is the HTTP response containing all the missing messages and their count.
type Response struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
}
