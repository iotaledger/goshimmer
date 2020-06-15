package metrics

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestActiveConflicts(t *testing.T) {
	// initialized to 0
	assert.Equal(t, ActiveConflicts(), (uint64)(0))

}
