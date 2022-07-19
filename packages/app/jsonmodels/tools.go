package jsonmodels

// PastconeRequest holds the block id to query.
type PastconeRequest struct {
	ID string `json:"id"`
}

// PastconeResponse is the HTTP response containing the number of blocks in the past cone and if all blocks of the past cone
// exist on the node.
type PastconeResponse struct {
	Exist        bool   `json:"exist,omitempty"`
	PastConeSize int    `json:"pastConeSize,omitempty"`
	Error        string `json:"error,omitempty"`
}

// MissingResponse is the HTTP response containing all the missing blocks and their count.
type MissingResponse struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
}

// MissingAvailableResponse is a map of blockIDs with the peers that have such block.
type MissingAvailableResponse struct {
	Availability map[string][]string `json:"blkavailability,omitempty"`
	Count        int                 `json:"count"`
}
