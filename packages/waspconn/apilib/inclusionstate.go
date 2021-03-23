package apilib

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type InclusionStateResponse struct {
	State ledgerstate.InclusionState `json:"state"`
	Err   string                     `json:"err"`
}

func InclusionState(netLoc string, txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	url := fmt.Sprintf("http://%s/utxodb/inclusionstate/%s", netLoc, txid.String())
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	res := &InclusionStateResponse{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK || res.Err != "" {
		return 0, fmt.Errorf("%s returned code %d: %s", url, resp.StatusCode, res.Err)
	}
	return res.State, nil
}
