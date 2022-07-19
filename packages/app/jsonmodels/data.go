package jsonmodels

// DataResponse contains the ID of the block sent.
type DataResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// DataRequest contains the data of the block to send.
type DataRequest struct {
	Data        []byte `json:"data"`
	MaxEstimate int64  `json:"maxEstimate"`
}
