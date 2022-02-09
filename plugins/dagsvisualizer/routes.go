package dagsvisualizer

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/cockroachdb/errors"
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
	app         = "/plugins/dagsvisualizer/frontend/build"
	staticJS    = "/plugins/dagsvisualizer/frontend/build/static/js"
	staticCSS   = "/plugins/dagsvisualizer/frontend/build/static/css"
	staticMedia = "/plugins/dagsvisualizer/frontend/build/static/media"
)

func indexRoute(e echo.Context) error {
	if Parameters.Dev {
		req, err := http.NewRequestWithContext(e.Request().Context(), "GET", "http://"+Parameters.DevBindAddress, nil)
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		devIndexHTML, err := io.ReadAll(res.Body)
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

	indexHTML, err := io.ReadAll(index)
	if err != nil {
		return err
	}
	return e.HTMLBlob(http.StatusOK, indexHTML)
}

func setupRoutes(e *echo.Echo) {
	if err := prepareSources(e); err != nil {
		return
	}

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the dashboard index
	e.GET("*", indexRoute)

	apiRoutes := e.Group("/api")

	setupDagsVisualizerRoutes(apiRoutes)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Warnf("Request failed: %s", err)

		var statusCode int
		var message string

		switch errors.Unwrap(err) {
		case echo.ErrNotFound:
			if e := c.Redirect(http.StatusSeeOther, "/"); e != nil {
				log.Warn("failed to redirect request")
			}
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
		if err := c.String(statusCode, message); err != nil {
			log.Warn("failed to send error response")
		}
	}
}

func prepareSources(e *echo.Echo) error {
	if Parameters.Dev {
		e.GET("/static/*", func(e echo.Context) error {
			res, err := http.Get("http://" + Parameters.DevBindAddress + e.Request().URL.Path)
			if err != nil {
				return err
			}
			defer res.Body.Close()

			devIndexHTML, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return e.HTMLBlob(http.StatusOK, devIndexHTML)
		})
	} else {
		// load assets from pkger: either from within the binary or actual disk
		err := pkger.Walk(app, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			e.GET("/app/"+info.Name(), echo.WrapHandler(http.StripPrefix("/app", http.FileServer(pkger.Dir(app)))))
			return nil
		})
		if err != nil {
			log.Warnf("Failed to load files in pkger: %s", err)
			return err
		}

		err = pkger.Walk(staticJS, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			e.GET("/static/js/"+info.Name(), echo.WrapHandler(http.StripPrefix("/static/js/", http.FileServer(pkger.Dir(staticJS)))))
			return nil
		})
		if err != nil {
			log.Warnf("Failed to load js files in pkger: %s", err)
			return err
		}

		err = pkger.Walk(staticCSS, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			e.GET("/static/css/"+info.Name(), echo.WrapHandler(http.StripPrefix("/static/css/", http.FileServer(pkger.Dir(staticCSS)))))
			return nil
		})
		if err != nil {
			log.Warnf("Failed to load css files in pkger: %s", err)
			return err
		}

		err = pkger.Walk(staticMedia, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			e.GET("/static/media/"+info.Name(), echo.WrapHandler(http.StripPrefix("/static/media/", http.FileServer(pkger.Dir(staticMedia)))))
			return nil
		})
		if err != nil {
			log.Warnf("Failed to load media files in pkger: %s", err)
			return err
		}
	}
	return nil
}
