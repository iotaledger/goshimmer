package webauth

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/dgrijalva/jwt-go"
)

var secret = "secret"

func configure(plugin *node.Plugin) {

	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey != "" {
		secret = jwtKey
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
	daemon.BackgroundWorker("webauth", func(shutdownSignal <-chan struct{}) {
		webapi.Server.GET("login", func(c echo.Context) error {
			username := c.FormValue("username")
			password := c.FormValue("password")
			uiUser := os.Getenv("UI_USER")
			uiPass := os.Getenv("UI_PASS")

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
	}, shutdown.ShutdownPriorityWebAPI)
}

// PLUGIN plugs the UI into the main program
var PLUGIN = node.NewPlugin("webauth", node.Disabled, configure, run)
