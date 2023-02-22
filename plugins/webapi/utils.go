package webapi

import (
	"encoding/json"
	"io"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// ParseJSONRequest parses json from HTTP request body into the dest.
func ParseJSONRequest(c echo.Context, dest interface{}) error {
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(dest); err != nil && !errors.Is(err, io.EOF) {
		return errors.Wrapf(err, "can't parse request body as json into %T", dest)
	}
	return nil
}
