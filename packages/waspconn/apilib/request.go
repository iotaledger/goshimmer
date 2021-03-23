package apilib

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type RequestFundsResponse struct {
	Err string `json:"err"`
}

func RequestFunds(netLoc string, address ledgerstate.Address) error {
	url := fmt.Sprintf("http://%s/utxodb/requestfunds/%s", netLoc, address.String())
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	res := &RequestFundsResponse{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK || res.Err != "" {
		return fmt.Errorf("%s returned code %d: %s", url, resp.StatusCode, res.Err)
	}
	return nil
}
