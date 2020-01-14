// Implements a very simple wrapper for GoShimmer's HTTP API .
package shimmer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/plugins/webapi/broadcastData"
	webapi_findTransactions "github.com/iotaledger/goshimmer/plugins/webapi/findTransactions"
	"github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"
	webapi_getTransactions "github.com/iotaledger/goshimmer/plugins/webapi/getTransactions"
	"github.com/iotaledger/goshimmer/plugins/webapi/getTrytes"
	webapi_gtta "github.com/iotaledger/goshimmer/plugins/webapi/gtta"
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

func NewShimmerAPI(node string) *ShimmerAPI {
	return &ShimmerAPI{node: node}
}

type ShimmerAPI struct {
	http.Client
	node string
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

	if res.StatusCode == http.StatusOK {
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

func (api *ShimmerAPI) BroadcastData(targetAddress trinary.Trytes, data trinary.Trytes) (trinary.Hash, error) {
	if !guards.IsHash(targetAddress) {
		return "", errors.Wrapf(consts.ErrInvalidHash, "invalid address: %s", targetAddress)
	}
	if !guards.IsTrytes(data) {
		return "", errors.Wrapf(consts.ErrInvalidTrytes, "invalid trytes: %s", data)
	}

	reqBytes, err := json.Marshal(&broadcastData.Request{Address: targetAddress, Data: data})
	if err != nil {
		return "", err
	}

	res, err := api.Post(fmt.Sprintf("%s/%s", api.node, routeBroadcastData), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}

	resObj := &broadcastData.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return "", err
	}

	return resObj.Hash, nil
}

func (api *ShimmerAPI) GetTrytes(txHashes trinary.Hashes) ([]trinary.Trytes, error) {
	for _, hash := range txHashes {
		if !guards.IsTrytes(hash) {
			return nil, errors.Wrapf(consts.ErrInvalidHash, "invalid hash: %s", hash)
		}
	}

	reqBytes, err := json.Marshal(&getTrytes.Request{Hashes: txHashes})
	if err != nil {
		return nil, err
	}

	res, err := api.Post(fmt.Sprintf("%s/%s", api.node, routeGetTrytes), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	resObj := &getTrytes.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj.Trytes, nil
}

func (api *ShimmerAPI) GetTransactions(txHashes trinary.Hashes) ([]webapi_getTransactions.Transaction, error) {
	for _, hash := range txHashes {
		if !guards.IsTrytes(hash) {
			return nil, errors.Wrapf(consts.ErrInvalidHash, "invalid hash: %s", hash)
		}
	}

	reqBytes, err := json.Marshal(&webapi_getTransactions.Request{Hashes: txHashes})
	if err != nil {
		return nil, err
	}

	res, err := api.Post(fmt.Sprintf("%s/%s", api.node, routeGetTransactions), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_getTransactions.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj.Transactions, nil
}

func (api *ShimmerAPI) FindTransactions(query *webapi_findTransactions.Request) ([]trinary.Hashes, error) {
	for _, hash := range query.Addresses {
		if !guards.IsTrytes(hash) {
			return nil, errors.Wrapf(consts.ErrInvalidHash, "invalid hash: %s", hash)
		}
	}

	reqBytes, err := json.Marshal(&query)
	if err != nil {
		return nil, err
	}

	res, err := api.Post(fmt.Sprintf("%s/%s", api.node, routeFindTransactions), contentTypeJSON, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_findTransactions.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj.Transactions, nil
}

func (api *ShimmerAPI) GetNeighbors() (*getNeighbors.Response, error) {
	res, err := api.Get(fmt.Sprintf("%s/%s", api.node, routeGetNeighbors))
	if err != nil {
		return nil, err
	}

	resObj := &getNeighbors.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj, nil
}

func (api *ShimmerAPI) GetTips() (*webapi_gtta.Response, error) {
	res, err := api.Get(fmt.Sprintf("%s/%s", api.node, routeGetTransactionsToApprove))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_gtta.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj, nil
}

func (api *ShimmerAPI) ToggleSpammer(enable bool) (*webapi_gtta.Response, error) {
	res, err := api.Get(fmt.Sprintf("%s/%s?cmd=%s", api.node, routeSpammer, func() string {
		if enable {
			return "start"
		}
		return "stop"
	}()))
	if err != nil {
		return nil, err
	}

	resObj := &webapi_gtta.Response{}
	if err := interpretBody(res, resObj); err != nil {
		return nil, err
	}

	return resObj, nil
}
