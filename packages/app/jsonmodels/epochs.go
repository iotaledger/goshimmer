package jsonmodels

type Commitment struct {
	ID                   string `json:"commitmentID"`
	Index                int64  `json:"index"`
	LatestConfirmedIndex int64  `json:"latestConfirmedEI"`
	PrevID               string `json:"prevID"`
	RootsID              string `json:"commitmentIDBytes"`
	CumulativeWeight     int64  `json:"cumulativeWeight"`
	Bytes                []byte `json:"bytes"`
}
