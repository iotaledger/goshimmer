package message

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"

	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// MissingHandler process missing requests.
func MissingHandler(c echo.Context) error {
	res := &jsonmodels2.MissingResponse{}
	missingIDs := messagelayer.Tangle().Storage.MissingMessages()
	for _, msg := range missingIDs {
		res.IDs = append(res.IDs, msg.Base58())
	}
	res.Count = len(missingIDs)
	return c.JSON(http.StatusOK, res)
}
