package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
)

const (
	routeBlock         = "blocks/"
	routeBlockMetadata = "/metadata"
	routeSendPayload   = "blocks/payload"
	routeSendBlock     = "blocks"
	routeGetReferences = "blocks/references"
)

// GetBlock is the handler for the /blocks/:blockID endpoint.
func (api *GoShimmerAPI) GetBlock(base58EncodedID string) (*jsonmodels.Block, error) {
	res := &jsonmodels.Block{}

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
func (api *GoShimmerAPI) GetBlockMetadata(base58EncodedID string) (*retainer.BlockMetadata, error) {
	res := retainer.NewBlockMetadata()

	if err := api.do(
		http.MethodGet,
		routeBlock+base58EncodedID+routeBlockMetadata,
		nil,
		res,
	); err != nil {
		return nil, err
	}

	res.SetID(res.M.ID)

	return res, nil
}

// SendPayload send a block with the given payload.
func (api *GoShimmerAPI) SendPayload(payload []byte) (string, error) {
	res := &jsonmodels.PostPayloadResponse{}
	if err := api.do(http.MethodPost, routeSendPayload,
		&jsonmodels.PostPayloadRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}

// SendBlock sends a block provided in form of bytes.
func (api *GoShimmerAPI) SendBlock(blockBytes []byte) (string, error) {
	res := &jsonmodels.PostBlockResponse{}
	if err := api.do(http.MethodPost, routeSendBlock,
		&jsonmodels.PostBlockRequest{BlockBytes: blockBytes}, res); err != nil {
		return "", err
	}
	return res.BlockID, nil
}

// GetReferences returns the parent references selected by the node for a given payload.
func (api *GoShimmerAPI) GetReferences(payload []byte, parentsCount int) (resp *jsonmodels.GetReferencesResponse, err error) {
	res := &jsonmodels.GetReferencesResponse{}
	if err := api.do(http.MethodGet, routeGetReferences,
		&jsonmodels.GetReferencesRequest{
			PayloadBytes: payload,
			ParentsCount: parentsCount,
		}, res); err != nil {
		return nil, err
	}

	return res, nil
}
