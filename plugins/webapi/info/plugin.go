package info

import (
	"encoding/hex"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
)

var PLUGIN = node.NewPlugin("WebAPI info Endpoint", node.Enabled, configure)

func configure(plugin *node.Plugin) {
	webapi.Server.GET("info", getInfo)
}

// getInfo returns the info of the node
func getInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, Response{PublicKey: hex.EncodeToString(local.GetInstance().PublicKey().Bytes())})
}

type Response struct {
	PublicKey string `json:"publickey,omitempty"`
	Error     string `json:"error,omitempty"`
}
