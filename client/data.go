package client

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
)

const (
	routeData = "data"
)

// Data sends the given data (payload) by creating a block in the backend.
func (api *GoShimmerAPI) Data(data []byte, maxWait ...time.Duration) (string, error) {
	res := &jsonmodels.DataResponse{}
	dataRequest := &jsonmodels.DataRequest{Data: data}
	if len(maxWait) > 0 {
		dataRequest = &jsonmodels.DataRequest{Data: data, MaxEstimate: maxWait[0].Milliseconds()}
	}
	if err := api.do(http.MethodPost, routeData, dataRequest, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
