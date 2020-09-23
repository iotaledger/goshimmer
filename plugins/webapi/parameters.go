package webapi

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgBindAddress defines the config flag of the web API binding address.
	CfgBindAddress = "webapi.bindAddress"
	// CfgDisabledAPI defines the config flag of disabled web APIs.
	CfgDisabledAPI = "webapi.disabledapi"
)

func init() {
	flag.String(CfgBindAddress, "127.0.0.1:8080", "the bind address for the web API")
	flag.String(CfgDisabledAPI, "", "the disabled web API list, the apis are disabled by its root of route")
}
