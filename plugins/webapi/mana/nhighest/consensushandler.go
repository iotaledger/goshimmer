package nhighest

import (
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/labstack/echo"
)

// ConsensusHandler handles the request.
func ConsensusHandler(c echo.Context) error {
	return Handler(c, mana.ConsensusMana)
}
