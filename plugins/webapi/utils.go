package webapi

import (
	"encoding/json"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
)

// ParseJSONRequest parses json from HTTP request body into the dest.
func ParseJSONRequest(c echo.Context, dest interface{}) error {
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(dest); err != nil && !errors.Is(err, io.EOF) {
		return errors.Errorf("can't parse request body as json into %T: %w", dest, err)
	}
	return nil
}
