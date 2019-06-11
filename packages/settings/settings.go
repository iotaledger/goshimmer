package settings

import (
	"github.com/iotaledger/goshimmer/packages/database"
	"sync"
)

var settingsDatabase database.Database

var lazyInit sync.Once

func Get(key []byte) ([]byte, error) {
	lazyInit.Do(initDb)

	return settingsDatabase.Get(key)
}

func Set(key []byte, value []byte) error {
	lazyInit.Do(initDb)

	return settingsDatabase.Set(key, value)
}

func initDb() {
	if db, err := database.Get("settings"); err != nil {
		panic(err)
	} else {
		settingsDatabase = db
	}
}
