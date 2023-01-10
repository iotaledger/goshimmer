package dashboard

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/cockroachdb/errors"
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

//go:embed frontend/build frontend/src/assets
var staticFS embed.FS

const (
	app    = "frontend/build"
	assets = "frontend/src/assets"
)

func indexRoute(e echo.Context) error {
	if Parameters.Dev {
		fmt.Println("in dev mode")
		req, err := http.NewRequestWithContext(e.Request().Context(), "GET", Parameters.DevDashboardAddress, nil /* body */)
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
	if Parameters.Dev {
		e.Static("/assets", "./plugins/dashboard/frontend/src/assets")
	} else {
		staticfsys := fs.FS(staticFS)

		appfs, _ := fs.Sub(staticfsys, app)
		dirEntries, _ := staticFS.ReadDir(app)
		for _, de := range dirEntries {
			e.GET("/app/"+de.Name(), echo.WrapHandler(http.StripPrefix("/app", http.FileServer(http.FS(appfs)))))
		}

		assetsfs, _ := fs.Sub(staticfsys, assets)
		dirEntries, _ = staticFS.ReadDir(assets)
		for _, de := range dirEntries {
			e.GET("/assets/"+de.Name(), echo.WrapHandler(http.StripPrefix("/assets", http.FileServer(http.FS(assetsfs)))))
		}
	}

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the dashboard index
	e.GET("*", indexRoute)

	apiRoutes := e.Group("/api")

	setupExplorerRoutes(apiRoutes)
	setupVisualizerRoutes(apiRoutes)
	setupTipsRoutes(apiRoutes)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Warnf("Request failed: %s", err)

		var statusCode int
		var block string

		switch errors.UnwrapAll(err) {
		case echo.ErrNotFound:
			c.Redirect(http.StatusSeeOther, "/")
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
		c.String(statusCode, block)
	}
}
