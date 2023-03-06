package dpos

import (
	"time"

	"github.com/iotaledger/hive.go/runtime/options"
)

// WithActivityWindow sets the duration for which a validator is recognized as active after issuing a block.
func WithActivityWindow(activityWindow time.Duration) options.Option[SybilProtection] {
	return func(p *SybilProtection) {
		p.optsActivityWindow = activityWindow
	}
}
