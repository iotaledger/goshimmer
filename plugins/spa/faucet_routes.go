package spa

import (
	"net/http"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

type SendResult struct {
	Resp string `json:"res"`
}

func setupFaucetRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/faucet/:hash", func(c echo.Context) (err error) {
		address := c.Param("hash")
		if len(address) < 81 {
			return errors.Wrapf(ErrInvalidParameter, "search address invalid: %s", address)
		}

		t, err := sendFaucetReq(address)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})
}

func sendFaucetReq(address string) (res *SendResult, err error) {
	r := &SendResult{
		Resp: "sentFaucet",
	}
	return r, nil
	//TODO
	// Send request here
}
