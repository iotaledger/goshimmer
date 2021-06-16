package database_test

import (
	"os"
	"testing"

	. "github.com/iotaledger/goshimmer/packages/testhelper"
)

// TestMain will setup and teardown any unit test
func TestMain(m *testing.M) {
	GlobalSetup()
	code := m.Run()
	GlobalTeardown()
	os.Exit(code)
}
