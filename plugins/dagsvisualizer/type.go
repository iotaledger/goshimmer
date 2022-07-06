package dagsvisualizer

import "github.com/iotaledger/goshimmer/packages/jsonmodels"

const (
	// MsgTypeTangleVertex is the type of the Tangle DAG vertex.
	MsgTypeTangleVertex byte = iota
	// MsgTypeTangleBooked is the type of the Tangle DAG confirmed message.
	MsgTypeTangleBooked
	// MsgTypeTangleConfirmed is the type of the Tangle DAG confirmed message.
	MsgTypeTangleConfirmed
	// MsgTypeTangleTxConfirmationState is the type of the Tangle DAG transaction ConfirmationState.
	MsgTypeTangleTxConfirmationState
	// MsgTypeUTXOVertex is the type of the UTXO DAG vertex.
	MsgTypeUTXOVertex
	// MsgTypeUTXOBooked is the type of the booked transaction.
	MsgTypeUTXOBooked
	// MsgTypeUTXOConfirmationStateChanged is the type of the UTXO DAG vertex confirmation state message.
	MsgTypeUTXOConfirmationStateChanged
	// MsgTypeBranchVertex is the type of the branch DAG vertex.
	MsgTypeBranchVertex
	// MsgTypeBranchParentsUpdate is the type of the branch DAG vertex parents updated message.
	MsgTypeBranchParentsUpdate
	// MsgTypeBranchConfirmationStateChanged is the type of the branch DAG vertex confirmed message.
	MsgTypeBranchConfirmationStateChanged
	// MsgTypeBranchWeightChanged is the type of the branch DAG vertex weight changed message.
	MsgTypeBranchWeightChanged
)

type wsMessage struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type tangleVertex struct {
	ID                    string   `json:"ID"`
	StrongParentIDs       []string `json:"strongParentIDs"`
	WeakParentIDs         []string `json:"weakParentIDs"`
	ShallowLikeParentIDs  []string `json:"shallowLikeParentIDs"`
	BranchIDs             []string `json:"branchIDs"`
	IsMarker              bool     `json:"isMarker"`
	IsTx                  bool     `json:"isTx"`
	TxID                  string   `json:"txID,omitempty"`
	IsConfirmed           bool     `json:"isConfirmed"`
	ConfirmationStateTime int64    `json:"confirmationStateTime"`
	ConfirmationState     string   `json:"confirmationState,omitempty"`
}

type tangleBooked struct {
	ID        string   `json:"ID"`
	IsMarker  bool     `json:"isMarker"`
	BranchIDs []string `json:"branchIDs"`
}

type tangleConfirmed struct {
	ID                    string `json:"ID"`
	ConfirmationState     string `json:"confirmationState"`
	ConfirmationStateTime int64  `json:"confirmationStateTime"`
}

type tangleTxConfirmationStateChanged struct {
	ID          string `json:"ID"`
	IsConfirmed bool   `json:"isConfirmed"`
}

type utxoVertex struct {
	MsgID                 string              `json:"msgID"`
	ID                    string              `json:"ID"`
	Inputs                []*jsonmodels.Input `json:"inputs"`
	Outputs               []string            `json:"outputs"`
	IsConfirmed           bool                `json:"isConfirmed"`
	ConfirmationState     string              `json:"confirmationState"`
	BranchIDs             []string            `json:"branchIDs"`
	ConfirmationStateTime int64               `json:"confirmationStateTime"`
}

type utxoBooked struct {
	ID        string   `json:"ID"`
	BranchIDs []string `json:"branchIDs"`
}

type utxoConfirmationStateChanged struct {
	ID                    string `json:"ID"`
	ConfirmationState     string `json:"confirmationState"`
	ConfirmationStateTime int64  `json:"confirmationStateTime"`
	IsConfirmed           bool   `json:"isConfirmed"`
}

type branchVertex struct {
	ID                string                                 `json:"ID"`
	Parents           []string                               `json:"parents"`
	IsConfirmed       bool                                   `json:"isConfirmed"`
	Conflicts         *jsonmodels.GetBranchConflictsResponse `json:"conflicts"`
	ConfirmationState string                                 `json:"confirmationState"`
	AW                float64                                `json:"aw"`
}

type branchParentUpdate struct {
	ID      string   `json:"ID"`
	Parents []string `json:"parents"`
}

type branchConfirmationStateChanged struct {
	ID                string `json:"ID"`
	ConfirmationState string `json:"confirmationState"`
	IsConfirmed       bool   `json:"isConfirmed"`
}

type branchWeightChanged struct {
	ID                string  `json:"ID"`
	Weight            float64 `json:"weight"`
	ConfirmationState string  `json:"confirmationState"`
}

type searchResult struct {
	Messages []*tangleVertex `json:"messages"`
	Txs      []*utxoVertex   `json:"txs"`
	Branches []*branchVertex `json:"branches"`
	Error    string          `json:"error,omitempty"`
}
