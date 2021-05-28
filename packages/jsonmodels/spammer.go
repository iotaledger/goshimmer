package jsonmodels

// SpammerResponse is the HTTP response of a spammer request.
type SpammerResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// SpammerRequest contains the parameters of a spammer request.
type SpammerRequest struct {
	Cmd  string `json:"cmd"`
	IMIF string `json:"imif"`
	MPM  int    `json:"mpm"`
}
