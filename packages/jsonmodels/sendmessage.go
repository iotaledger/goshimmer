package jsonmodels

// ParentMessageIDs is a json representation of tangle.ParentMessageIDs.
type ParentMessageIDs struct {
	Type       uint8    `json:"type"`
	MessageIDs []string `json:"messageIDs"`
}

// SendMessageRequest contains the data of send message endpoint.
type SendMessageRequest struct {
	Payload          []byte             `json:"payload"`
	ParentMessageIDs []ParentMessageIDs `json:"parentMessageIDs"`
}
