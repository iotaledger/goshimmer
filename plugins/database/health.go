package database

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/database"
)

var (
	healthStore kvstore.KVStore
	healthKey   = []byte("db_health")
)

func configureHealthStore(store kvstore.KVStore) {
	var err error
	healthStore, err = store.WithRealm([]byte{database.PrefixHealth})
	if err != nil {
		panic(err)
	}
}

// MarkDatabaseUnhealthy marks the database as not healthy, meaning
// that it wasn't shutdown properly.
func MarkDatabaseUnhealthy() {
	if err := healthStore.Set(healthKey, []byte{}); err != nil {
		panic(fmt.Errorf("failed to set database health state: %w", err))
	}
}

// MarkDatabaseHealthy marks the database as healthy, respectively correctly closed.
func MarkDatabaseHealthy() {
	if err := healthStore.Delete(healthKey); err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(fmt.Errorf("failed to set database health state: %w", err))
	}
}

// IsDatabaseUnhealthy tells whether the database is unhealthy, meaning not shutdown properly.
func IsDatabaseUnhealthy() bool {
	contains, err := healthStore.Has(healthKey)
	if err != nil {
		panic(fmt.Errorf("failed to set database health state: %w", err))
	}
	return contains
}
