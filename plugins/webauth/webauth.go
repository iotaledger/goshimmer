package webauth

import (
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/dgrijalva/jwt-go"
)

var PLUGIN = node.NewPlugin("WebAPI Auth", node.Disabled, configure)
var log *logger.Logger
var privateKey string

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("WebAPI Auth")
	privateKey = parameter.NodeConfig.GetString(WEBAPI_AUTH_PRIVATE_KEY)
	if len(privateKey) == 0 {
		panic("")
	}

	webapi.Server.Use(middleware.JWTWithConfig(middleware.JWTConfig{
		SigningKey: []byte(privateKey),
		Skipper: func(c echo.Context) bool {
			if strings.HasPrefix(c.Path(), "/ui") || c.Path() == "/login" {
				return true
			}
			return false
		},
	}))

	webapi.Server.POST("/login", Handler)
	log.Info("WebAPI is now secured through JWT authentication")
}

type Request struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Response struct {
	Token string `json:"token"`
}

func Handler(c echo.Context) error {
	login := &Request{}
	if err := c.Bind(login); err != nil {
		return echo.ErrBadRequest
	}

	if login.Username != parameter.NodeConfig.GetString(WEBAPI_AUTH_USERNAME) ||
		login.Password != parameter.NodeConfig.GetString(WEBAPI_AUTH_PASSWORD) {
		return echo.ErrUnauthorized
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["name"] = login.Username
	claims["exp"] = time.Now().Add(time.Hour * 24 * 7).Unix()

	t, err := token.SignedString([]byte(privateKey))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, &Response{Token: t})
}
