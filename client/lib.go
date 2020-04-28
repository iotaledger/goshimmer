// Package client implements a very simple wrapper for GoShimmer's web API.
package client

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
	// ErrBadRequest defines the "bad request" error.
	ErrBadRequest = errors.New("bad request")
	// ErrInternalServerError defines the "internal server error" error.
	ErrInternalServerError = errors.New("internal server error")
	// ErrNotFound defines the "not found" error.
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized defines the "unauthorized" error.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrUnknownError defines the "unknown error" error.
	ErrUnknownError = errors.New("unknown error")
	// ErrNotImplemented defines the "operation not implemented/supported/available" error.
	ErrNotImplemented = errors.New("operation not implemented/supported/available")
)

const (
	contentTypeJSON = "application/json"
)

// NewGoShimmerAPI returns a new *GoShimmerAPI with the given baseURL and httpClient.
func NewGoShimmerAPI(baseURL string, httpClient ...http.Client) *GoShimmerAPI {
	if len(httpClient) > 0 {
		return &GoShimmerAPI{baseURL: baseURL, httpClient: httpClient[0]}
	}
	return &GoShimmerAPI{baseURL: baseURL}
}

// GoShimmerAPI is an API wrapper over the web API of GoShimmer.
type GoShimmerAPI struct {
	httpClient http.Client
	baseURL    string
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
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", api.baseURL, route), func() io.Reader {
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

// BaseURL returns the baseURL of the API.
func (api *GoShimmerAPI) BaseURL() string {
	return api.baseURL
}
