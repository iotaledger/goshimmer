package webapi

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
)

// ParseJSONRequest parses json from HTTP request body into the dest.
func ParseJSONRequest(c echo.Context, dest interface{}) error {
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(dest); err != nil {
		return errors.Errorf("can't parse request body as json into %T: %w", dest, err)
	}
	return nil
}
