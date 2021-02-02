package fcob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFCoB(t *testing.T) {
	now := time.Now()
	conflictSet := ConflictSet{&Opinion{
		Timestamp:        now,
		Liked:            false,
		LevelOfKnowledge: Pending,
	}}

	opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
	assert.Equal(t, &Opinion{
		Timestamp:        now.Add(1 * time.Second),
		Liked:            false,
		LevelOfKnowledge: One,
	}, opinion)
}
