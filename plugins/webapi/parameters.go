package webapi

import (
	flag "github.com/spf13/pflag"
)

const (
	BIND_ADDRESS = "webapi.bindAddress"
)

func init() {
	flag.String(BIND_ADDRESS, "0.0.0.0:8080", "the bind address for the web API")
}
