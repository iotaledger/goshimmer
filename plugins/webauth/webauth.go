package webauth

import (
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/dgrijalva/jwt-go"
)

var secret string

func configure(plugin *node.Plugin) {

	secret = parameter.NodeConfig.GetString(UI_JWTKEY)
	if secret == "" {
		secret = randString(16)
	}

	webapi.Server.Use(middleware.JWTWithConfig(middleware.JWTConfig{
		SigningKey:  []byte(secret),
		TokenLookup: "query:token",
		Skipper: func(c echo.Context) bool {
			// if strings.HasPrefix(c.Request().Host, "localhost") {
			// 	return true
			// }
			if strings.HasPrefix(c.Path(), "/ui") || c.Path() == "/login" {
				return true
			}
			return false
		},
	}))
}

func run(plugin *node.Plugin) {
	daemon.BackgroundWorker("webauth", func() {
		webapi.AddEndpoint("login", func(c echo.Context) error {
			username := c.FormValue("username")
			password := c.FormValue("password")
			uiUser := parameter.NodeConfig.GetString(UI_USER)
			uiPass := parameter.NodeConfig.GetString(UI_PASS)

			// Throws unauthorized error
			if username != uiUser || password != uiPass {
				return echo.ErrUnauthorized
			}

			token := jwt.New(jwt.SigningMethodHS256)
			claims := token.Claims.(jwt.MapClaims)
			claims["name"] = username
			claims["exp"] = time.Now().Add(time.Hour * 24 * 7).Unix()

			t, err := token.SignedString([]byte(secret))
			if err != nil {
				return err
			}

			return c.JSON(http.StatusOK, map[string]string{
				"token": t,
			})
		})
	})
}

// PLUGIN plugs the UI into the main program
var PLUGIN = node.NewPlugin("webauth", node.Disabled, configure, run)

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
