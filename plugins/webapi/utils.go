package webapi

import (
	"encoding/json"

	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

// ParseJSONRequest parses json from HTTP request body into the dest.
func ParseJSONRequest(c echo.Context, dest interface{}) error {
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(dest); err != nil {
		return xerrors.Errorf("can't parse request body as json into %T: %w", dest, err)
	}
	return nil
}
