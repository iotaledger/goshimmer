package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
)

const (
	routeLatestCommitment = "commitments/latest"
	routeCommitment       = "commitments/"
)

func (api *GoShimmerAPI) GetLatestCommitment() (*jsonmodels.Commitment, error) {
	res := &jsonmodels.Commitment{}
	if err := api.do(http.MethodGet, routeLatestCommitment, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (api *GoShimmerAPI) GetCommitment(epochIndex string) (*jsonmodels.Commitment, error) {
	res := &jsonmodels.Commitment{}
	if err := api.do(
		http.MethodGet,
		routeCommitment+epochIndex,
		nil,
		res,
	); err != nil {
		return nil, err
	}
	return res, nil
}
