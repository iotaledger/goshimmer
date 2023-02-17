package dashboard

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// ErrInvalidParameter defines the invalid parameter error.
var ErrInvalidParameter = echo.NewHTTPError(http.StatusBadRequest, "invalid parameter")

// ErrInternalError defines the internal error.
var ErrInternalError = echo.ErrInternalServerError

// ErrNotFound defines the not found error.
var ErrNotFound = echo.ErrNotFound

// ErrForbidden defines the forbidden error.
var ErrForbidden = echo.ErrForbidden

//go:embed frontend/build frontend/src/assets
var staticFS embed.FS

const (
	app    = "frontend/build"
	assets = "frontend/src/assets"
)

func indexRoute(e echo.Context) error {
	if Parameters.Dev {
		req, err := http.NewRequestWithContext(e.Request().Context(), "GET", "http://"+Parameters.DevDashboardAddress, http.NoBody)
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
		var message string

		var e *echo.HTTPError
		if errors.As(err, &e) {
			if e.Code == http.StatusNotFound {
				if redirectErr := c.Redirect(http.StatusSeeOther, "/"); redirectErr != nil {
					log.Warnf("failed to redirect request: %s", redirectErr.Error())
				}
				return
			}

			statusCode = e.Code
			message = fmt.Sprintf("%s, error: %s", e.Message, err)
		} else {
			statusCode = http.StatusInternalServerError
			message = fmt.Sprintf("internal server error. error: %s", err)
		}

		resErr := c.String(statusCode, message)
		if resErr != nil {
			log.Warnf("Failed to send error response: %s", resErr)
		}
	}
}
