package jsonmodels

// SpammerResponse is the HTTP response of a spammer request.
type SpammerResponse struct {
	Block string `json:"block"`
	Error string `json:"error"`
}

// SpammerRequest contains the parameters of a spammer request.
type SpammerRequest struct {
	Cmd         string `query:"cmd"`
	IMIF        string `query:"imif"`
	Rate        int    `query:"rate"`
	Unit        string `query:"unit"`
	PayloadSize uint64 `query:"payloadSize"`
}
