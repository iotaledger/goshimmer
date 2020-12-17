package dashboard

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/labstack/echo"
	"github.com/markbates/pkger"
)

// ErrInvalidParameter defines the invalid parameter error.
var ErrInvalidParameter = errors.New("invalid parameter")

// ErrInternalError defines the internal error.
var ErrInternalError = errors.New("internal error")

// ErrNotFound defines the not found error.
var ErrNotFound = errors.New("not found")

// ErrForbidden defines the forbidden error.
var ErrForbidden = errors.New("forbidden")

const (
	app    = "/plugins/dashboard/frontend/build"
	assets = "/plugins/dashboard/frontend/src/assets"
)

func indexRoute(e echo.Context) error {
	if config.Node().Bool(CfgDev) {
		res, err := http.Get("http://127.0.0.1:9090/")
		if err != nil {
			return err
		}
		devIndexHTML, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return e.HTMLBlob(http.StatusOK, devIndexHTML)
	}

	index, err := pkger.Open(app + "/index.html")
	if err != nil {
		return err
	}
	defer index.Close()

	indexHTML, err := ioutil.ReadAll(index)
	if err != nil {
		return err
	}
	return e.HTMLBlob(http.StatusOK, indexHTML)
}

func setupRoutes(e *echo.Echo) {

	if config.Node().Bool("dashboard.dev") {
		e.Static("/assets", "./plugins/dashboard/frontend/src/assets")
	} else {

		// load assets from pkger: either from within the binary or actual disk
		pkger.Walk(app, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			e.GET("/app/"+info.Name(), echo.WrapHandler(http.StripPrefix("/app", http.FileServer(pkger.Dir(app)))))
			return nil
		})

		pkger.Walk(assets, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			e.GET("/assets/"+info.Name(), echo.WrapHandler(http.StripPrefix("/assets", http.FileServer(pkger.Dir(assets)))))
			return nil
		})
	}

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the dashboard index
	e.GET("*", indexRoute)

	apiRoutes := e.Group("/api")

	setupExplorerRoutes(apiRoutes)
	setupFaucetRoutes(apiRoutes)
	setupVisualizerRoutes(apiRoutes)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Warnf("Request failed: %s", err)

		var statusCode int
		var message string

		switch errors.Unwrap(err) {

		case echo.ErrNotFound:
			c.Redirect(http.StatusSeeOther, "/")
			return

		case echo.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
			message = "unauthorized"

		case ErrForbidden:
			statusCode = http.StatusForbidden
			message = "access forbidden"

		case ErrInternalError:
			statusCode = http.StatusInternalServerError
			message = "internal server error"

		case ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case ErrInvalidParameter:
			statusCode = http.StatusBadRequest
			message = "bad request"

		default:
			statusCode = http.StatusInternalServerError
			message = "internal server error"
		}

		message = fmt.Sprintf("%s, error: %+v", message, err)
		c.String(statusCode, message)
	}
}
