package throughputquota

import (
	"github.com/iotaledger/hive.go/core/identity"
)

type ThroughputQuota interface {
	// Init initializes the ThroughputQuota plugin and is called after all remaining dependencies have been injected.
	Init()

	// Balance returns the balance of the given identity.
	Balance(id identity.ID) (mana int64, exists bool)

	// BalanceByIDs returns the balances of all known identities.
	BalanceByIDs() (quotaByID map[identity.ID]int64)

	// TotalBalance returns the total amount of throughput quota.
	TotalBalance() (totalQuota int64)
}
