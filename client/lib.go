// Implements a very simple wrapper for GoShimmer's HTTP API .
package goshimmer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/errors"
	webapi_broadcastData "github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	webapi_findTransactions "github.com/iotaledger/goshimmer/plugins/webapi/findTransactions"
	webapi_getNeighbors "github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"
	webapi_getTransactions "github.com/iotaledger/goshimmer/plugins/webapi/getTransactions"
	webapi_getTrytes "github.com/iotaledger/goshimmer/plugins/webapi/getTrytes"
	webapi_gtta "github.com/iotaledger/goshimmer/plugins/webapi/gtta"
	webapi_spammer "github.com/iotaledger/goshimmer/plugins/webapi/spammer"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	ErrBadRequest          = errors.New("bad request")
	ErrInternalServerError = errors.New("internal server error")
	ErrNotFound            = errors.New("not found")
	ErrUnknownError        = errors.New("unknown error")
)

const (
	routeBroadcastData            = "broadcastData"
	routeGetTrytes                = "getTrytes"
	routeGetTransactions          = "getTransactions"
	routeFindTransactions         = "findTransactions"
	routeGetNeighbors             = "getNeighbors"
	routeGetTransactionsToApprove = "getTransactionsToApprove"
	routeSpammer                  = "spammer"

	contentTypeJSON = "application/json"
)

func NewGoShimmerAPI(node string, httpClient ...http.Client) *GoShimmerAPI {
	if len(httpClient) > 0 {
		return &GoShimmerAPI{node: node, httpClient: httpClient[0]}
	}
	return &GoShimmerAPI{node: node}
}

type GoShimmerAPI struct {
	httpClient http.Client
	node       string
}

type errorresponse struct {
	Error string `json:"error"`
}

func interpretBody(res *http.Response, decodeTo interface{}) error {
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "unable to read response body")
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		return json.Unmarshal(resBody, decodeTo)
	}

	errRes := &errorresponse{}
	if err := json.Unmarshal(resBody, errRes); err != nil {
		return errors.Wrap(err, "unable to read error from response body")
	}

	switch res.StatusCode {
	case http.StatusInternalServerError:
		return errors.Wrap(ErrInternalServerError, errRes.Error)
	case http.StatusNotFound:
		return errors.Wrap(ErrNotFound, errRes.Error)
	case http.StatusBadRequest:
		return errors.Wrap(ErrBadRequest, errRes.Error)
	}
	return errors.Wrap(ErrUnknownError, errRes.Error)
}

func (api *GoShimmerAPI) BroadcastData(targetAddress trinary.Trytes, data string) (trinary.Hash, error) {
	if !guards.IsHash(targetAddress) {
		return "", errors.Wrapf(consts.ErrInvalidHash, "invalid address: %s", targetAddress)
	}

	reqBytes, err := json.Marshal(&webapi_broadcastData.Request{Address: targetAddress, Data: data})
	if err != nil {
		return "", err
	}

	res, err := api.httpClient.Post(fmt.Sprintf("%s/%s", api.node, routeBroadcastData), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}

	resObj := &webapi_broadcastData.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return "", err
	}

	return resObj.Hash, nil
}

func (api *GoShimmerAPI) GetTrytes(txHashes trinary.Hashes) ([]trinary.Trytes, error) {
	for _, hash := range txHashes {
		if !guards.IsTrytes(hash) {
			return nil, errors.Wrapf(consts.ErrInvalidHash, "invalid hash: %s", hash)
		}
	}

	reqBytes, err := json.Marshal(&webapi_getTrytes.Request{Hashes: txHashes})
	if err != nil {
		return nil, err
	}

	res, err := api.httpClient.Post(fmt.Sprintf("%s/%s", api.node, routeGetTrytes), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_getTrytes.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj.Trytes, nil
}

func (api *GoShimmerAPI) GetTransactions(txHashes trinary.Hashes) ([]webapi_getTransactions.Transaction, error) {
	for _, hash := range txHashes {
		if !guards.IsTrytes(hash) {
			return nil, errors.Wrapf(consts.ErrInvalidHash, "invalid hash: %s", hash)
		}
	}

	reqBytes, err := json.Marshal(&webapi_getTransactions.Request{Hashes: txHashes})
	if err != nil {
		return nil, err
	}

	res, err := api.httpClient.Post(fmt.Sprintf("%s/%s", api.node, routeGetTransactions), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_getTransactions.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj.Transactions, nil
}

func (api *GoShimmerAPI) FindTransactions(query *webapi_findTransactions.Request) ([]trinary.Hashes, error) {
	for _, hash := range query.Addresses {
		if !guards.IsTrytes(hash) {
			return nil, errors.Wrapf(consts.ErrInvalidHash, "invalid hash: %s", hash)
		}
	}

	reqBytes, err := json.Marshal(&query)
	if err != nil {
		return nil, err
	}

	res, err := api.httpClient.Post(fmt.Sprintf("%s/%s", api.node, routeFindTransactions), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_findTransactions.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj.Transactions, nil
}


func (api *GoShimmerAPI) GetNeighbors() (*webapi_getNeighbors.Response, error) {
	res, err := api.httpClient.Get(fmt.Sprintf("%s/%s", api.node, routeGetNeighbors))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_getNeighbors.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj, nil
}


func (api *GoShimmerAPI) GetTips() (*webapi_gtta.Response, error) {
	res, err := api.httpClient.Get(fmt.Sprintf("%s/%s", api.node, routeGetTransactionsToApprove))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_gtta.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj, nil
}

func (api *GoShimmerAPI) ToggleSpammer(enable bool) (*webapi_spammer.Response, error) {
	res, err := api.httpClient.Get(fmt.Sprintf("%s/%s?cmd=%s", api.node, routeSpammer, func() string {
		if enable {
			return "start"
		}
		return "stop"
	}()))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_spammer.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj, nil
}
