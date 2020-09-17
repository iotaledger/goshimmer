package nhighest

import (
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/labstack/echo"
)

// AccessHandler handles the request.
func AccessHandler(c echo.Context) error {
	return Handler(c, mana.AccessMana)
}
