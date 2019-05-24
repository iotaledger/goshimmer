package parameters

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	QUORUMSIZE = parameter.AddInt("FPC/QUORUMSIZE", 3, "size of Quorum for FPC")
	ROUND_TIME = parameter.AddInt("FPC/ROUND_TIME", 5, "Time of Round")
)
