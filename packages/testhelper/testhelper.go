package testhelper

import (
	"flag"

	"github.com/iotaledger/goshimmer/packages/database"
)

// global cache time flag
var (
	cacheFlag = flag.Int("database.globalCacheTime", 0, "number of seconds all objects should remain in memory."+
		" -1 means use defaults")
)

// GlobalSetup setups global environment and parameters for any unit test
func GlobalSetup() {
	flag.Parse()

	setGlobalCacheTime()
}

// GlobalTeardown resets all the changes the test made
func GlobalTeardown() {
	// To be implemented
}

func setGlobalCacheTime() {
	database.SetGlobalCacheTimeOnce(*cacheFlag)
}
