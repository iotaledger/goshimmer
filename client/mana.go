package client

import (
	"fmt"
	"net/http"

	webapi_mana "github.com/iotaledger/goshimmer/plugins/webapi/mana"
)

const (
	routeGetMana                  = "mana"
	routeGetAllMana               = "mana/all"
	routeGetManaPercentile        = "mana/percentile"
	routeGetOnlineAccessMana      = "mana/access/online"
	routeGetOnlineConsensusMana   = "mana/consensus/online"
	routeGetNHighestAccessMana    = "mana/access/nhighest"
	routeGetNHighestConsensusMana = "mana/consensus/nhighest"
	routePending                  = "mana/pending"
	routePastConsensusVector      = "mana/consensus/past"
	routePastConsensusEventLogs   = "mana/consensus/logs"
)

// GetOwnMana returns the access and consensus mana of the node this api client is communicating with.
func (api *GoShimmerAPI) GetOwnMana() (*webapi_mana.GetManaResponse, error) {
	res := &webapi_mana.GetManaResponse{}
	if err := api.do(http.MethodGet, routeGetMana,
		&webapi_mana.GetManaRequest{NodeID: ""}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetManaFullNodeID returns the access and consensus mana of the node specified in the argument.
// Note, that for the node to understand which nodeID we are referring to, short node ID is not sufficient.
func (api *GoShimmerAPI) GetManaFullNodeID(fullNodeID string) (*webapi_mana.GetManaResponse, error) {
	res := &webapi_mana.GetManaResponse{}
	if err := api.do(http.MethodGet, routeGetMana,
		&webapi_mana.GetManaRequest{NodeID: fullNodeID}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetMana returns the access and consensus mana a node has based on its shortNodeID.
func (api *GoShimmerAPI) GetMana(shortNodeID string) (*webapi_mana.GetManaResponse, error) {
	// ask the node about the full mana map and filter out based on shortID
	allManaRes := &webapi_mana.GetAllManaResponse{}
	if err := api.do(http.MethodGet, routeGetAllMana,
		nil, allManaRes); err != nil {
		return nil, err
	}
	res := &webapi_mana.GetManaResponse{ShortNodeID: shortNodeID}
	// look for node's mana values in the map
	for _, nodeStr := range allManaRes.Access {
		if nodeStr.ShortNodeID == shortNodeID {
			res.Access = nodeStr.Mana
			break
		}
	}
	for _, nodeStr := range allManaRes.Consensus {
		if nodeStr.ShortNodeID == shortNodeID {
			res.Consensus = nodeStr.Mana
			break
		}
	}
	return res, nil
}

// GetAllMana returns the mana perception of the node in the network.
func (api *GoShimmerAPI) GetAllMana() (*webapi_mana.GetAllManaResponse, error) {
	res := &webapi_mana.GetAllManaResponse{}
	if err := api.do(http.MethodGet, routeGetAllMana,
		nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetManaPercentile returns the mana percentile for access and consensus mana of a node.
func (api *GoShimmerAPI) GetManaPercentile(fullNodeID string) (*webapi_mana.GetPercentileResponse, error) {
	res := &webapi_mana.GetPercentileResponse{}
	if err := api.do(http.MethodGet, routeGetManaPercentile,
		&webapi_mana.GetPercentileRequest{NodeID: fullNodeID}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOnlineAccessMana returns the sorted list of online access mana of nodes.
func (api *GoShimmerAPI) GetOnlineAccessMana() (*webapi_mana.GetOnlineResponse, error) {
	res := &webapi_mana.GetOnlineResponse{}
	if err := api.do(http.MethodGet, routeGetOnlineAccessMana,
		nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOnlineConsensusMana returns the sorted list of online consensus mana of nodes.
func (api *GoShimmerAPI) GetOnlineConsensusMana() (*webapi_mana.GetOnlineResponse, error) {
	res := &webapi_mana.GetOnlineResponse{}
	if err := api.do(http.MethodGet, routeGetOnlineConsensusMana,
		nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetNHighestAccessMana returns the N highest access mana holders in the network, sorted in descending order.
func (api *GoShimmerAPI) GetNHighestAccessMana(n int) (*webapi_mana.GetNHighestResponse, error) {
	res := &webapi_mana.GetNHighestResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?number=%d", routeGetNHighestAccessMana, n)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetNHighestConsensusMana returns the N highest consensus mana holders in the network, sorted in descending order.
func (api *GoShimmerAPI) GetNHighestConsensusMana(n int) (*webapi_mana.GetNHighestResponse, error) {
	res := &webapi_mana.GetNHighestResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?number=%d", routeGetNHighestConsensusMana, n)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetPending returns the mana (bm2) that will be pledged by spending the output specified.
func (api *GoShimmerAPI) GetPending(outputID string) (*webapi_mana.PendingResponse, error) {
	res := &webapi_mana.PendingResponse{}
	if err := api.do(http.MethodGet, routePending,
		&webapi_mana.PendingRequest{OutputID: outputID}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetPastConsensusManaVector returns the consensus base mana vector of a time in the past.
func (api *GoShimmerAPI) GetPastConsensusManaVector(t int64) (*webapi_mana.PastConsensusManaVectorResponse, error) {
	res := &webapi_mana.PastConsensusManaVectorResponse{}
	if err := api.do(http.MethodGet, routePastConsensusVector,
		&webapi_mana.PastConsensusManaVectorRequest{Timestamp: t}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetPastConsensusVectorMetadata returns the consensus base mana vector metadata of a time in the past.
func (api *GoShimmerAPI) GetPastConsensusVectorMetadata() (*webapi_mana.PastConsensusVectorMetadataResponse, error) {
	res := &webapi_mana.PastConsensusVectorMetadataResponse{}
	if err := api.do(http.MethodGet, routePastConsensusVector, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConsensusEventLogs returns the consensus event logs or the nodeIDs specified.
func (api *GoShimmerAPI) GetConsensusEventLogs(nodeIDs []string) (*webapi_mana.GetEventLogsResponse, error) {
	res := &webapi_mana.GetEventLogsResponse{}
	if err := api.do(http.MethodGet, routePastConsensusEventLogs,
		&webapi_mana.GetEventLogsRequest{NodeIDs: nodeIDs}, res); err != nil {
		return nil, err
	}
	return res, nil
}
