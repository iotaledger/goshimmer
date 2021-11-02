// Package client implements a very simple wrapper for GoShimmer's web API.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
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
	contentType     = "Content-Type"
	contentTypeJSON = "application/json"
	contentTypeCSV  = "text/csv"
)

// Option is a function which sets the given option.
type Option func(*GoShimmerAPI)

// BasicAuth defines the basic-auth struct.
type BasicAuth struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// WithBasicAuth returns a new BasicAuth.
func WithBasicAuth(username, password string) Option {
	return func(g *GoShimmerAPI) {
		g.basicAuth = BasicAuth{
			Enabled:  true,
			Username: username,
			Password: password,
		}
	}
}

// WithHTTPClient sets the http Client.
func WithHTTPClient(c http.Client) Option {
	return func(g *GoShimmerAPI) {
		g.httpClient = c
	}
}

// IsEnabled returns the enabled state of a given BasicAuth.
func (b BasicAuth) IsEnabled() bool {
	return b.Enabled
}

// Credentials returns the username and password of a given BasicAuth.
func (b BasicAuth) Credentials() (username, password string) {
	return b.Username, b.Password
}

// NewGoShimmerAPI returns a new *GoShimmerAPI with the given baseURL and options.
func NewGoShimmerAPI(baseURL string, setters ...Option) *GoShimmerAPI {
	g := &GoShimmerAPI{
		baseURL: baseURL,
	}
	for _, setter := range setters {
		setter(g)
	}
	return g
}

// GoShimmerAPI is an API wrapper over the web API of GoShimmer.
type GoShimmerAPI struct {
	baseURL    string
	httpClient http.Client
	basicAuth  BasicAuth
}

type errorresponse struct {
	Error string `json:"error"`
}

func interpretBody(res *http.Response, decodeTo interface{}) error {
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("unable to read response body: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		switch contType := res.Header.Get(contentType); {
		case strings.HasPrefix(contType, contentTypeJSON):
			return json.Unmarshal(resBody, decodeTo)
		case strings.HasPrefix(contType, contentTypeCSV):
			*decodeTo.(*csv.Reader) = *csv.NewReader(bufio.NewReader(bytes.NewReader(resBody)))
			return nil
		default:
			return fmt.Errorf("can't decode %s content-type", contType)
		}
	}
	errRes := &errorresponse{}
	if err := json.Unmarshal(resBody, errRes); err != nil {
		return fmt.Errorf("unable to read error from response body: %w repsonseBody: %s", err, resBody)
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
	ctx := context.TODO()
	// construct request
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", api.baseURL, route), func() io.Reader {
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

	// if enabled, add the basic-auth
	if api.basicAuth.IsEnabled() {
		req.SetBasicAuth(api.basicAuth.Credentials())
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
