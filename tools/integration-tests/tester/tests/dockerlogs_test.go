package tests

import (
	"bufio"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDockerLogs simply verifies that a peer's log message contains "GoShimmer".
// Using the combination of logs and regular expressions can be useful to test a certain peer's behavior.
func TestDockerLogs(t *testing.T) {
	n := f.CreateNetwork("TestDockerLogs", 3, 1)
	defer n.Shutdown()

	r := regexp.MustCompile("GoShimmer")

	for _, p := range n.Peers() {
		log := p.Logs()

		assert.True(t, r.MatchReader(bufio.NewReader(log)))
	}
}
