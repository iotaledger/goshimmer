package dagsvisualizer

const (
	// MsgTypeTangleVertex is the type of the Tangle DAG vertex.
	MsgTypeTangleVertex byte = iota
	// MsgTypeTangleBooked is the type of the Tangle DAG confirmed message.
	MsgTypeTangleBooked
	// MsgTypeTangleConfirmed is the type of the Tangle DAG confirmed message.
	MsgTypeTangleConfirmed
	// MsgTypeFutureMarkerUpdated is the type of the future marker updated message.
	MsgTypeFutureMarkerUpdated
	// MsgTypeMarkerAWUpdated is the type of the marker's AW updated message.
	MsgTypeMarkerAWUpdated
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
	IsMarker        bool     `json:"ismarker"`
	ApprovalWeight  float64  `json:"approvalweight"`
	ConfirmedTime   int64    `json:"confirmedTime"`
}

type tangleBooked struct {
	ID       string `json:"ID"`
	BranchID string `json:"branchID"`
}

type tangleFinalized struct {
	ID            string `json:"ID"`
	ConfirmedTime int64  `json:"confirmedTime"`
}

type tangleFutureMarkerUpdated struct {
	ID             string `json:"ID"`
	FutureMarkerID string `json:"futureMarkerID"`
}

type tangleMarkerAWUpdated struct {
	ID             string  `jsong:"ID"`
	ApprovalWeight float64 `json:"approvalweight"`
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
