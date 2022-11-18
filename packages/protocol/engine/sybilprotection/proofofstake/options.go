package proofofstake

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
)

// WithActivityWindow sets the duration for which a validator is recognized as active after issuing a block.
func WithActivityWindow(activityWindow time.Duration) options.Option[ProofOfStake] {
	return func(p *ProofOfStake) {
		p.optsActivityWindow = activityWindow
	}
}
