package testutil

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var dbInstance *badger.DB

func DB(t *testing.T) *badger.DB {
	if dbInstance == nil {
		// Set up DB for testing
		dir, err := ioutil.TempDir("", t.Name())
		require.NoError(t, err)

		t.Cleanup(func() {
			os.Remove(dir)

			dbInstance = nil
		})

		// use the tempdir for the database
		config.Node.Set(database.CFG_DIRECTORY, dir)

		dbInstance = database.GetBadgerInstance()
	}

	return dbInstance
}
