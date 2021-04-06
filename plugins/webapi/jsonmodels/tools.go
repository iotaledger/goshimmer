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

// ObjectsResponse is the HTTP response from retrieving value objects.
type ObjectsResponse struct {
	ValueObjects []Object `json:"value_objects,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Object holds the info of a value object
type Object struct {
	Parents        []string `json:"parent,omitempty"`
	ID             string   `json:"id"`
	Tip            bool     `json:"tip,omitempty"`
	InclusionState string   `json:"inclusionState"`
	Solid          bool     `json:"solid"`
	Finalized      bool     `json:"finalize"`
	Rejected       bool     `json:"rejected"`
	BranchID       string   `json:"branch_id"`
	TransactionID  string   `json:"transaction_id"`
}

// MissingResponse is the HTTP response containing all the missing messages and their count.
type MissingResponse struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
}
