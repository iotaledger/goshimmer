package webauth

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgWebAPIAuthUsername defines the config flag of the web API authentication username.
	CfgWebAPIAuthUsername = "webapi.auth.username"
	// CfgWebAPIAuthPassword defines the config flag of the web API authentication password.
	CfgWebAPIAuthPassword = "webapi.auth.password"
	// CfgWebAPIAuthPrivateKey defines the config flag of the web API authentication private key.
	CfgWebAPIAuthPrivateKey = "webapi.auth.privateKey"
)

func init() {
	flag.String(CfgWebAPIAuthUsername, "goshimmer", "username for the webapi")
	flag.String(CfgWebAPIAuthPassword, "goshimmer", "password for the webapi")
	flag.String(CfgWebAPIAuthPrivateKey, "", "private key used to sign the JWTs")
}
