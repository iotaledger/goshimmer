package dagsvisualizer

const (
	// MsgTypeTangleVertex is the type of the Tangle DAG vertex.
	MsgTypeTangleVertex byte = iota
	// MsgTypeTangleBooked is the type of the Tangle DAG confirmed message.
	MsgTypeTangleBooked
	// MsgTypeTangleConfirmed is the type of the Tangle DAG confirmed message.
	MsgTypeTangleConfirmed
	// MsgTypeUTXOVertex is the type of the UTXO DAG vertex.
	MsgTypeUTXOVertex
	// MsgTypeUTXOConfirmed is the type of the UTXO DAG vertex confirmed message.
	MsgTypeUTXOConfirmed
	// MsgTypeBranchVertex is the type of the branch DAG vertex.
	MsgTypeBranchVertex
	// MsgTypeBranchParentsUpdate is the type of the branch DAG vertex parents updated message.
	MsgTypeBranchParentsUpdate
)

type wsMessage struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type tangleVertex struct {
	ID              string   `json:"ID"`
	StrongParentIDs []string `json:"strongParentIDs"`
	WeakParentIDs   []string `json:"weakParentIDs"`
	ApprovalWeight  float64  `json:"approvalweight"`
	ConfirmedTime   int64    `json:"confirmedTime"`
}

type tangleBooked struct {
	ID       string `json:"ID"`
	BranchID string `json:"branchID"`
}

type tangleFinalized struct {
	ID             string  `json:"ID"`
	ApprovalWeight float64 `json:"approvalweight"`
	ConfirmedTime  int64   `json:"confirmedTime"`
}

type utxoVertex struct {
	MsgID          string   `json:"msgID"`
	ID             string   `json:"ID"`
	Inputs         []string `json:"inputs"`
	Outputs        []string `json:"outputs"`
	ApprovalWeight float64  `json:"approvalweight"`
	ConfirmedTime  int64    `json:"confirmedTime"`
}

type utxoConfirmed struct {
	ID             string  `json:"ID"`
	ApprovalWeight float64 `json:"approvalweight"`
	ConfirmedTime  int64   `json:"confirmedTime"`
}

type branchVertex struct {
	ID             string   `json:"ID"`
	Parents        []string `json:"parents"`
	ApprovalWeight float64  `json:"approvalweight"`
	ConfirmedTime  int64    `json:"confirmedTime"`
}

type branchParentUpdate struct {
	ID      string   `json:"ID"`
	Parents []string `json:"parents"`
}
