package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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
	routeBroadcastData = "broadcastData"
	routeGetNeighbors  = "getNeighbors"

	contentTypeJSON = "application/json"
)

func New(baseUrl string, httpClient http.Client) *Api {
	return &Api{BaseUrl: baseUrl, httpClient: httpClient}
}

// Api is an API wrapper over the web API of GoShimmer.
type Api struct {
	httpClient http.Client
	BaseUrl    string
	jwt        string
}

type errorResponse struct {
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

	errRes := &errorResponse{}
	if err := json.Unmarshal(resBody, errRes); err != nil {
		return fmt.Errorf("unable to read error from response body: %w", err)
	}

	switch res.StatusCode {
	case http.StatusInternalServerError:
		return fmt.Errorf("%w: %s", ErrInternalServerError, errRes.Error)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, res.Request.URL.String())
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, errRes.Error)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, errRes.Error)
	case http.StatusNotImplemented:
		return fmt.Errorf("%w: %s", ErrNotImplemented, errRes.Error)
	}

	return fmt.Errorf("%w: %s", ErrUnknownError, errRes.Error)
}

func (api *Api) do(method string, route string, reqObj interface{}, resObj interface{}) error {
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
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", api.BaseUrl, route), func() io.Reader {
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

func (api *Api) BroadcastData(data string) (string, error) {
	res := &BroadcastDataResponse{}

	err := api.do(
		http.MethodPost,
		routeBroadcastData,
		&BroadcastDataRequest{Data: data},
		res,
	)
	if err != nil {
		return "", err
	}

	return res.Hash, nil
}

// GetNeighbors gets the chosen/accepted neighbors.
// If knownPeers is set, also all known peers to the node are returned additionally.
func (api *Api) GetNeighbors(knownPeers bool) (*GetNeighborResponse, error) {
	res := &GetNeighborResponse{}

	err := api.do(
		http.MethodGet,
		func() string {
			if !knownPeers {
				return routeGetNeighbors
			}
			return fmt.Sprintf("%s?known=1", routeGetNeighbors)
		}(),
		nil,
		res,
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}
