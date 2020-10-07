package webapi

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgBindAddress defines the config flag of the web API binding address.
	CfgBindAddress = "webapi.bindAddress"
	// CfgBasicAuthEnabled defines the config flag of the webapi basic auth enabler.
	CfgBasicAuthEnabled = "webapi.basic_auth.enabled"
	// CfgBasicAuthUsername defines the config flag of the webapi basic auth username.
	CfgBasicAuthUsername = "webapi.basic_auth.username"
	// CfgBasicAuthPassword defines the config flag of the webapi basic auth password.
	CfgBasicAuthPassword = "webapi.basic_auth.password"
)

func init() {
	flag.String(CfgBindAddress, "127.0.0.1:8080", "the bind address for the web API")
	flag.Bool(CfgBasicAuthEnabled, false, "whether to enable HTTP basic auth")
	flag.String(CfgBasicAuthUsername, "goshimmer", "HTTP basic auth username")
	flag.String(CfgBasicAuthPassword, "goshimmer", "HTTP basic auth password")
}
