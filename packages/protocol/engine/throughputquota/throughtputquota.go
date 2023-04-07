package throughputquota

import (
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/module"
)

type ThroughputQuota interface {
	// Balance returns the balance of the given identity.
	Balance(id identity.ID) (mana int64, exists bool)

	// BalanceByIDs returns the balances of all known identities.
	BalanceByIDs() (quotaByID map[identity.ID]int64)

	// TotalBalance returns the total amount of throughput quota.
	TotalBalance() (totalQuota int64)

	// Interface embeds the required methods of the module.Interface.
	module.Interface
}
