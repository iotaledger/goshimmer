package value

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
