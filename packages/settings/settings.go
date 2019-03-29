package settings

import "github.com/iotaledger/goshimmer/packages/database"

var settingsDatabase database.Database

func init() {
    if db, err := database.Get("settings"); err != nil {
        panic(err)
    } else {
        settingsDatabase = db
    }
}

func Get(key []byte) ([]byte, error) {
    return settingsDatabase.Get(key)
}

func Set(key []byte, value []byte) error {
    return settingsDatabase.Set(key, value)
}