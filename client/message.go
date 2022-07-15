package client

import (
	"net/http"

	jsonmodels2 "github.com/iotaledger/goshimmer/packages/models/jsonmodels"
)

const (
	routeBlock         = "blocks/"
	routeBlockMetadata = "/metadata"
	routeSendPayload   = "blocks/payload"
)

// GetBlock is the handler for the /blocks/:blockID endpoint.
func (api *GoShimmerAPI) GetBlock(base58EncodedID string) (*jsonmodels2.Block, error) {
	res := &jsonmodels2.Block{}

	if err := api.do(
		http.MethodGet,
		routeBlock+base58EncodedID,
		nil,
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}

// GetBlockMetadata is the handler for the /blocks/:blockID/metadata endpoint.
func (api *GoShimmerAPI) GetBlockMetadata(base58EncodedID string) (*jsonmodels2.BlockMetadata, error) {
	res := &jsonmodels2.BlockMetadata{}

	if err := api.do(
		http.MethodGet,
		routeBlock+base58EncodedID+routeBlockMetadata,
		nil,
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}

// SendPayload send a block with the given payload.
func (api *GoShimmerAPI) SendPayload(payload []byte) (string, error) {
	res := &jsonmodels2.PostPayloadResponse{}
	if err := api.do(http.MethodPost, routeSendPayload,
		&jsonmodels2.PostPayloadRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
