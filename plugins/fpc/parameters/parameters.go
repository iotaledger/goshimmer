package parameters

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
	QUORUMSIZE   = parameter.AddInt("FPC/QUORUMSIZE", 3, "size of Quorum for FPC")
	ROUND_TIME   = parameter.AddInt("FPC/ROUND_TIME", 5, "Time of Round")
	PRNG_ADDRESS = parameter.AddString("FPC/PRNG_ADDRESS", "127.0.0.1", "Centralized PRNG address")
	PRNG_PORT    = parameter.AddString("FPC/PRNG_PORT", "10000", "Centralized PRNG tcp port")
)
