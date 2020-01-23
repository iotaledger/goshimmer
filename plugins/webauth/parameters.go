package webauth

import (
	flag "github.com/spf13/pflag"
)

const (
	WEBAPI_AUTH_USERNAME    = "webapi.auth.username"
	WEBAPI_AUTH_PASSWORD    = "webapi.auth.password"
	WEBAPI_AUTH_PRIVATE_KEY = "webapi.auth.privateKey"
)

func init() {
	flag.String(WEBAPI_AUTH_USERNAME, "goshimmer", "username for the webapi")
	flag.String(WEBAPI_AUTH_PASSWORD, "goshimmer", "password for the webapi")
	flag.String(WEBAPI_AUTH_PRIVATE_KEY, "", "private key used to sign the JWTs")
}
