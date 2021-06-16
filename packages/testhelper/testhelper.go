package testhelper

import (
	"fmt"
	"os"
)

func GlobalSetup() {
	err := os.Setenv("GOSHIMMER_ENV", "testing")
	if err != nil {
		panic(fmt.Sprintf("Failed to set test environment variable. %s", err))
	}
}

// TestTeardown resets all the changes the test made
func GlobalTeardown() {
	err := os.Setenv("GOSHIMMER_ENV", "")
	if err != nil {
		panic(fmt.Sprintf("Failed to clear test environment variable. %s", err))
	}
}

// IsTest returns true if we are now running a unit test
func IsTest() bool {
	return os.Getenv("GOSHIMMER_ENV") == "testing"
}
