package jsonmodels

// PastconeRequest holds the message id to query.
type PastconeRequest struct {
	ID string `json:"id"`
}

// PastconeResponse is the HTTP response containing the number of messages in the past cone and if all messages of the past cone
// exist on the node.
type PastconeResponse struct {
	Exist        bool   `json:"exist,omitempty"`
	PastConeSize int    `json:"pastConeSize,omitempty"`
	Error        string `json:"error,omitempty"`
}

// MissingResponse is the HTTP response containing all the missing messages and their count.
type MissingResponse struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
}

// MissingAvailableResponse is a map of messageIDs with the peers that have such message.
type MissingAvailableResponse struct {
	Availability map[string][]string `json:"msgavailability,omitempty"`
	Count        int                 `json:"count"`
}
