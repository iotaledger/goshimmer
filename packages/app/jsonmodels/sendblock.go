package jsonmodels

// ParentBlockIDs is a json representation of tangleold.ParentBlockIDs.
type ParentBlockIDs struct {
	Type     uint8    `json:"type"`
	BlockIDs []string `json:"blockIDs"`
}

// SendBlockRequest contains the data of send block endpoint.
type SendBlockRequest struct {
	Payload        []byte           `json:"payload"`
	ParentBlockIDs []ParentBlockIDs `json:"parentBlockIDs"`
}
