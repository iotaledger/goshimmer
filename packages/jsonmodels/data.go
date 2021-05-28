package jsonmodels

// DataResponse contains the ID of the message sent.
type DataResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// DataRequest contains the data of the message to send.
type DataRequest struct {
	Data []byte `json:"data"`
}
