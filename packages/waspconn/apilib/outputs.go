package apilib

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
)

func GetAddressOutputs(netLoc string, address ledgerstate.Address) ([]value.OutputID, error) {
	url := fmt.Sprintf("http://%s/utxodb/outputs/%s", netLoc, address.Base58())
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	res := &value.UnspentOutputsResponse{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || res.Error != "" {
		return nil, fmt.Errorf("%s returned code %d -- %s", url, resp.StatusCode, res.Error)
	}
	if len(res.UnspentOutputs) != 1 || res.UnspentOutputs[0].Address != address.Base58() {
		return nil, fmt.Errorf("return value does not contain outputs for address")
	}

	return res.UnspentOutputs[0].OutputIDs, nil
}
