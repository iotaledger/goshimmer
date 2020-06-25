package client

import (
	"net/http"

	webapi_auth "github.com/iotaledger/goshimmer/plugins/webauth"
)

const (
	routeLogin = "login"
)

// Login authorizes this API instance against the web API.
// You must call this function before any other call, if the web-auth plugin is enabled.
func (api *GoShimmerAPI) Login(username string, password string) error {
	res := &webapi_auth.Response{}
	if err := api.do(http.MethodPost, routeLogin,
		&webapi_auth.Request{Username: username, Password: password}, res); err != nil {
		return err
	}
	api.jwt = res.Token
	return nil
}
