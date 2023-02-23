package dagsvisualizer

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// ErrInvalidParameter defines the invalid parameter error.
var ErrInvalidParameter = errors.New("invalid parameter")

// ErrInternalError defines the internal error.
var ErrInternalError = errors.New("internal error")

// ErrNotFound defines the not found error.
var ErrNotFound = errors.New("not found")

// ErrForbidden defines the forbidden error.
var ErrForbidden = errors.New("forbidden")

//go:embed frontend/build frontend/build/static
var staticFS embed.FS

const (
	app         = "frontend/build"
	staticJS    = "frontend/build/static/js"
	staticCSS   = "frontend/build/static/css"
	staticMedia = "frontend/build/static/media"
)

func indexRoute(e echo.Context) error {
	if Parameters.Dev {
		req, err := http.NewRequestWithContext(e.Request().Context(), "GET", "http://"+Parameters.DevBindAddress, http.NoBody)
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

	index, err := staticFS.Open(app + "/index.html")
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
	prepareSources(e)

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the dashboard index
	e.GET("*", indexRoute)

	apiRoutes := e.Group("/api")

	setupDagsVisualizerRoutes(apiRoutes)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Warnf("Request failed: %s", err)

		var statusCode int
		var block string

		switch errors.Unwrap(err) {
		case echo.ErrNotFound:
			if redirectErr := c.Redirect(http.StatusSeeOther, "/"); redirectErr != nil {
				log.Warnf("failed to redirect request: %s", redirectErr.Error())
			}
			return

		case echo.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
			block = "unauthorized"

		case ErrForbidden:
			statusCode = http.StatusForbidden
			block = "access forbidden"

		case ErrInternalError:
			statusCode = http.StatusInternalServerError
			block = "internal server error"

		case ErrNotFound:
			statusCode = http.StatusNotFound
			block = "not found"

		case ErrInvalidParameter:
			statusCode = http.StatusBadRequest
			block = "bad request"

		default:
			statusCode = http.StatusInternalServerError
			block = "internal server error"
		}

		block = fmt.Sprintf("%s, error: %+v", block, err)
		if err := c.String(statusCode, block); err != nil {
			log.Warn("failed to send error response")
		}
	}
}

func prepareSources(e *echo.Echo) {
	if Parameters.Dev {
		e.GET("/static/*", func(e echo.Context) error {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+Parameters.DevBindAddress+e.Request().URL.Path, http.NoBody)
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
		})
	} else {
		staticfsys := fs.FS(staticFS)

		appfs, _ := fs.Sub(staticfsys, app)
		dirEntries, _ := staticFS.ReadDir(app)
		for _, de := range dirEntries {
			e.GET("/app/"+de.Name(), echo.WrapHandler(http.StripPrefix("/app", http.FileServer(http.FS(appfs)))))
		}

		jsfs, _ := fs.Sub(staticfsys, staticJS)
		dirEntries, _ = staticFS.ReadDir(staticJS)
		for _, de := range dirEntries {
			e.GET("/static/js/"+de.Name(), echo.WrapHandler(http.StripPrefix("/static/js/", http.FileServer(http.FS(jsfs)))))
		}

		cssfs, _ := fs.Sub(staticfsys, staticCSS)
		dirEntries, _ = staticFS.ReadDir(staticCSS)
		for _, de := range dirEntries {
			e.GET("/static/css/"+de.Name(), echo.WrapHandler(http.StripPrefix("/static/css/", http.FileServer(http.FS(cssfs)))))
		}

		mediafs, _ := fs.Sub(staticfsys, staticMedia)
		dirEntries, _ = staticFS.ReadDir(staticMedia)
		for _, de := range dirEntries {
			e.GET("/static/media/"+de.Name(), echo.WrapHandler(http.StripPrefix("/static/media/", http.FileServer(http.FS(mediafs)))))
		}
	}
}
