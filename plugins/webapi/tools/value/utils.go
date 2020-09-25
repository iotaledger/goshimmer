package value

// Object holds the info of a value object
type Object struct {
	Parent1       string `json:"parent_1,omitempty"`
	Parent2       string `json:"parent_2,omitempty"`
	ID            string `json:"id"`
	Tip           bool   `json:"tip,omitempty"`
	Solid         bool   `json:"solid"`
	Liked         bool   `json:"liked"`
	Confirmed     bool   `json:"confirmed"`
	Rejected      bool   `json:"rejected"`
	BranchID      string `json:"branch_id"`
	TransactionID string `json:"transaction_id"`
}
