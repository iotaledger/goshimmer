package webauth

import (
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// PluginName is the name of the web API auth plugin.
const PluginName = "WebAPI Auth"

var (
	// Plugin is the plugin instance of the web API auth plugin.
	Plugin     = node.NewPlugin(PluginName, node.Disabled, configure)
	log        *logger.Logger
	privateKey string
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	privateKey = config.Node.GetString(CfgWebAPIAuthPrivateKey)
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

// Request defines the struct of the request.
type Request struct {
	// Username is the username of the request.
	Username string `json:"username"`
	// Password is the password of the request.
	Password string `json:"password"`
}

// Response defines the struct of the response.
type Response struct {
	// Token is the json web token.
	Token string `json:"token"`
}

// Handler handles the web auth request.
func Handler(c echo.Context) error {
	login := &Request{}
	if err := c.Bind(login); err != nil {
		return echo.ErrBadRequest
	}

	if login.Username != config.Node.GetString(CfgWebAPIAuthUsername) ||
		login.Password != config.Node.GetString(CfgWebAPIAuthPassword) {
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
