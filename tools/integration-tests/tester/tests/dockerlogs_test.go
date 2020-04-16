package tests

import (
	"bufio"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerLogs simply verifies that a peer's log message contains "GoShimmer".
// Using the combination of logs and regular expressions can be useful to test a certain peer's behavior.
func TestDockerLogs(t *testing.T) {
	r := regexp.MustCompile("GoShimmer")

	for _, p := range f.Peers() {
		log, err := p.Logs()
		require.NoError(t, err)

		assert.True(t, r.MatchReader(bufio.NewReader(log)))
	}
}
