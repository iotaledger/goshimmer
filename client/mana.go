package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
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
	routePastConsensusEventLogs   = "mana/consensus/logs"
	routeAllowedPledgeIssuerIDs   = "mana/allowedManaPledge"
)

// GetOwnMana returns the access and consensus mana of the issuer this api client is communicating with.
func (api *GoShimmerAPI) GetOwnMana() (*jsonmodels.GetManaResponse, error) {
	res := &jsonmodels.GetManaResponse{}
	if err := api.do(http.MethodGet, routeGetMana,
		&jsonmodels.GetManaRequest{IssuerID: ""}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetManaFullIssuerID returns the access and consensus mana of the issuer specified in the argument.
// Note, that for the issuer to understand which issuerID we are referring to, short issuer ID is not sufficient.
func (api *GoShimmerAPI) GetManaFullIssuerID(fullIssuerID string) (*jsonmodels.GetManaResponse, error) {
	res := &jsonmodels.GetManaResponse{}
	if err := api.do(http.MethodGet, routeGetMana,
		&jsonmodels.GetManaRequest{IssuerID: fullIssuerID}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetMana returns the access and consensus mana a issuer has based on its shortIssuerID.
func (api *GoShimmerAPI) GetMana(shortIssuerID string) (*jsonmodels.GetManaResponse, error) {
	// ask the issuer about the full mana map and filter out based on shortID
	allManaRes := &jsonmodels.GetAllManaResponse{}
	if err := api.do(http.MethodGet, routeGetAllMana,
		nil, allManaRes); err != nil {
		return nil, err
	}
	res := &jsonmodels.GetManaResponse{ShortIssuerID: shortIssuerID}
	// look for issuer's mana values in the map
	for _, issuerStr := range allManaRes.Access {
		if issuerStr.ShortIssuerID == shortIssuerID {
			res.Access = issuerStr.Mana
			break
		}
	}
	for _, issuerStr := range allManaRes.Consensus {
		if issuerStr.ShortIssuerID == shortIssuerID {
			res.Consensus = issuerStr.Mana
			break
		}
	}
	return res, nil
}

// GetAllMana returns the mana perception of the issuer in the network.
func (api *GoShimmerAPI) GetAllMana() (*jsonmodels.GetAllManaResponse, error) {
	res := &jsonmodels.GetAllManaResponse{}
	if err := api.do(http.MethodGet, routeGetAllMana,
		nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetManaPercentile returns the mana percentile for access and consensus mana of a issuer.
func (api *GoShimmerAPI) GetManaPercentile(fullIssuerID string) (*jsonmodels.GetPercentileResponse, error) {
	res := &jsonmodels.GetPercentileResponse{}
	if err := api.do(http.MethodGet, routeGetManaPercentile,
		&jsonmodels.GetPercentileRequest{IssuerID: fullIssuerID}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOnlineAccessMana returns the sorted list of online access mana of issuers.
func (api *GoShimmerAPI) GetOnlineAccessMana() (*jsonmodels.GetOnlineResponse, error) {
	res := &jsonmodels.GetOnlineResponse{}
	if err := api.do(http.MethodGet, routeGetOnlineAccessMana,
		nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOnlineConsensusMana returns the sorted list of online consensus mana of issuers.
func (api *GoShimmerAPI) GetOnlineConsensusMana() (*jsonmodels.GetOnlineResponse, error) {
	res := &jsonmodels.GetOnlineResponse{}
	if err := api.do(http.MethodGet, routeGetOnlineConsensusMana,
		nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetNHighestAccessMana returns the N highest access mana holders in the network, sorted in descending order.
func (api *GoShimmerAPI) GetNHighestAccessMana(n int) (*jsonmodels.GetNHighestResponse, error) {
	res := &jsonmodels.GetNHighestResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?number=%d", routeGetNHighestAccessMana, n)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetNHighestConsensusMana returns the N highest consensus mana holders in the network, sorted in descending order.
func (api *GoShimmerAPI) GetNHighestConsensusMana(n int) (*jsonmodels.GetNHighestResponse, error) {
	res := &jsonmodels.GetNHighestResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?number=%d", routeGetNHighestConsensusMana, n)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConsensusEventLogs returns the consensus event logs or the issuerIDs specified.
func (api *GoShimmerAPI) GetConsensusEventLogs(issuerIDs []string) (*jsonmodels.GetEventLogsResponse, error) {
	res := &jsonmodels.GetEventLogsResponse{}
	if err := api.do(http.MethodGet, routePastConsensusEventLogs,
		&jsonmodels.GetEventLogsRequest{IssuerIDs: issuerIDs}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetAllowedManaPledgeIssuerIDs returns the list of allowed mana pledge IDs.
func (api *GoShimmerAPI) GetAllowedManaPledgeIssuerIDs() (*jsonmodels.AllowedManaPledgeResponse, error) {
	res := &jsonmodels.AllowedManaPledgeResponse{}
	if err := api.do(http.MethodGet, routeAllowedPledgeIssuerIDs, nil, res); err != nil {
		return nil, err
	}

	return res, nil
}
