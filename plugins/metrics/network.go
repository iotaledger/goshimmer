package metrics

import "sync/atomic"

var (
	_FPCInboundBytes  *uint64
	_FPCOutboundBytes *uint64
)

func FPCInboundBytes() uint64 {
	return atomic.LoadUint64(_FPCInboundBytes)
}

func FPCOutboundBytes() uint64 {
	return atomic.LoadUint64(_FPCOutboundBytes)
}
