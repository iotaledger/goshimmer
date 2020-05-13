package dashboard

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gobuffalo/packr/v2"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/labstack/echo"
)

// ErrInvalidParameter defines the invalid parameter error.
var ErrInvalidParameter = errors.New("invalid parameter")

// ErrInternalError defines the internal error.
var ErrInternalError = errors.New("internal error")

// ErrNotFound defines the not found error.
var ErrNotFound = errors.New("not found")

// ErrForbidden defines the forbidden error.
var ErrForbidden = errors.New("forbidden")

// holds dashboard assets
var appBox = packr.New("Dashboard_App", "./frontend/build")
var assetsBox = packr.New("Dashboard_Assets", "./frontend/src/assets")

func indexRoute(e echo.Context) error {
	if config.Node.GetBool(CfgDev) {
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
	indexHTML, err := appBox.Find("index.html")
	if err != nil {
		return err
	}
	return e.HTMLBlob(http.StatusOK, indexHTML)
}

func setupRoutes(e *echo.Echo) {

	if config.Node.GetBool("dashboard.dev") {
		e.Static("/assets", "./plugins/dashboard/frontend/src/assets")
	} else {

		// load assets from packr: either from within the binary or actual disk
		for _, res := range appBox.List() {
			e.GET("/app/"+res, echo.WrapHandler(http.StripPrefix("/app", http.FileServer(appBox))))
		}

		for _, res := range assetsBox.List() {
			e.GET("/assets/"+res, echo.WrapHandler(http.StripPrefix("/assets", http.FileServer(assetsBox))))
		}
	}

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the dashboard index
	e.GET("*", indexRoute)

	apiRoutes := e.Group("/api")

	setupExplorerRoutes(apiRoutes)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		c.Logger().Error(err)

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
