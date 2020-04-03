// Implements a very simple wrapper for GoShimmer's web API .
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	webapi_broadcastData "github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	webapi_findMessageById "github.com/iotaledger/goshimmer/plugins/webapi/findMessageById"
	webapi_getNeighbors "github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"
	webapi_spammer "github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	webapi_auth "github.com/iotaledger/goshimmer/plugins/webauth"
)

var (
	ErrBadRequest          = errors.New("bad request")
	ErrInternalServerError = errors.New("internal server error")
	ErrNotFound            = errors.New("not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrUnknownError        = errors.New("unknown error")
	ErrNotImplemented      = errors.New("operation not implemented/supported/available")
)

const (
	routeBroadcastData  = "broadcastData"
	routeGetMessageById = "findMessageById"
	routeGetNeighbors   = "getNeighbors"
	routeSpammer        = "spammer"
	routeLogin          = "login"
	//routeGetTransactionsToApprove = "getTransactionsToApprove"

	contentTypeJSON = "application/json"
)

func NewGoShimmerAPI(node string, httpClient ...http.Client) *GoShimmerAPI {
	if len(httpClient) > 0 {
		return &GoShimmerAPI{baseUrl: node, httpClient: httpClient[0]}
	}
	return &GoShimmerAPI{baseUrl: node}
}

// GoShimmerAPI is an API wrapper over the web API of GoShimmer.
type GoShimmerAPI struct {
	httpClient http.Client
	baseUrl    string
	jwt        string
}

type errorresponse struct {
	Error string `json:"error"`
}

func interpretBody(res *http.Response, decodeTo interface{}) error {
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("unable to read response body: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		return json.Unmarshal(resBody, decodeTo)
	}

	errRes := &errorresponse{}
	if err := json.Unmarshal(resBody, errRes); err != nil {
		return fmt.Errorf("unable to read error from response body: %w", err)
	}

	switch res.StatusCode {
	case http.StatusInternalServerError:
		return fmt.Errorf("%w: %s", ErrInternalServerError, errRes.Error)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, res.Request.URL)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, errRes.Error)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, errRes.Error)
	case http.StatusNotImplemented:
		return fmt.Errorf("%w: %s", ErrNotImplemented, errRes.Error)
	}

	return fmt.Errorf("%w: %s", ErrUnknownError, errRes.Error)
}

func (api *GoShimmerAPI) do(method string, route string, reqObj interface{}, resObj interface{}) error {
	// marshal request object
	var data []byte
	if reqObj != nil {
		var err error
		data, err = json.Marshal(reqObj)
		if err != nil {
			return err
		}
	}

	// construct request
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", api.baseUrl, route), func() io.Reader {
		if data == nil {
			return nil
		}
		return bytes.NewReader(data)
	}())
	if err != nil {
		return err
	}

	if data != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	// add authorization header with JWT
	if len(api.jwt) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", api.jwt))
	}

	// make the request
	res, err := api.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resObj == nil {
		return nil
	}

	// write response into response object
	if err := interpretBody(res, resObj); err != nil {
		return err
	}
	return nil
}

// Login authorizes this API instance against the web API.
// You must call this function before any before any other call, if the web-auth plugin is enabled.
func (api *GoShimmerAPI) Login(username string, password string) error {
	res := &webapi_auth.Response{}
	if err := api.do(http.MethodPost, routeLogin,
		&webapi_auth.Request{Username: username, Password: password}, res); err != nil {
		return err
	}
	api.jwt = res.Token
	return nil
}

// BroadcastData sends the given data (payload) by creating a message in the backend.
func (api *GoShimmerAPI) BroadcastData(data []byte) (string, error) {

	res := &webapi_broadcastData.Response{}
	if err := api.do(http.MethodPost, routeBroadcastData,
		&webapi_broadcastData.Request{Data: data}, res); err != nil {
		return "", err
	}

	return res.Id, nil
}

func (api *GoShimmerAPI) FindMessageById(base58EncodedIds []string) (*webapi_findMessageById.Response, error) {
	res := &webapi_findMessageById.Response{}

	err := api.do(
		http.MethodPost,
		routeGetMessageById,
		&webapi_findMessageById.Request{Ids: base58EncodedIds},
		res,
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// GetNeighbors gets the chosen/accepted neighbors.
// If knownPeers is set, also all known peers to the node are returned additionally.
func (api *GoShimmerAPI) GetNeighbors(knownPeers bool) (*webapi_getNeighbors.Response, error) {
	res := &webapi_getNeighbors.Response{}
	if err := api.do(http.MethodGet, func() string {
		if !knownPeers {
			return routeGetNeighbors
		}
		return fmt.Sprintf("%s?known=1", routeGetNeighbors)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// // GetTips executes the tip-selection on the node to retrieve tips to approve.
// func (api *GoShimmerAPI) GetTransactionsToApprove() (*webapi_gtta.Response, error) {
// 	res := &webapi_gtta.Response{}
// 	if err := api.do(http.MethodGet, routeGetTransactionsToApprove, nil, res); err != nil {
// 		return nil, err
// 	}
// 	return res, nil
// }

// ToggleSpammer toggles the node internal spammer.
func (api *GoShimmerAPI) ToggleSpammer(enable bool) (*webapi_spammer.Response, error) {
	res := &webapi_spammer.Response{}
	if err := api.do(http.MethodGet, func() string {
		if enable {
			return fmt.Sprintf("%s?cmd=start", routeSpammer)
		}
		return fmt.Sprintf("%s?cmd=stop", routeSpammer)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *GoShimmerAPI) BaseUrl() string {
	return api.baseUrl
}
