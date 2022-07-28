package causalorder

import "github.com/iotaledger/goshimmer/packages/core/epoch"

type ID interface {
	comparable

	Index() epoch.Index
}
