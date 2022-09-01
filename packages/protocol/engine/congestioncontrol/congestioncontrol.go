package congestioncontrol

import (
	"github.com/iotaledger/hive.go/core/generics/options"
)

type CongestionControl struct {
}

func New(opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	return options.Apply(new(CongestionControl), opts, func(c *CongestionControl) {
		// TODO: init
	})
}
