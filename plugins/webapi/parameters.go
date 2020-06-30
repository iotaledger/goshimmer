package webapi

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgBindAddress defines the config flag of the web API binding address.
	CfgBindAddress = "webapi.bindAddress"
)

func init() {
	flag.String(CfgBindAddress, "127.0.0.1:8080", "the bind address for the web API")
}
