package fcob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeriveOpinion(t *testing.T) {
	now := time.Now()

	// 1 pending -> Dislike, One
	{
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

	// 1 Like, One -> Dislike, One
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            true,
				LevelOfKnowledge: One,
			}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}, opinion)
		}
	}

	// 1 Like, Two -> Dislike, Two
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            true,
				LevelOfKnowledge: Two,
			}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: Two,
			}, opinion)
		}
	}

	// 1 Dislike, Two -> Dislike, Pending
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            false,
				LevelOfKnowledge: Two,
			}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: Pending,
			}, opinion)
		}
	}

	// 1 Dislike, One -> Dislike, One
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            false,
				LevelOfKnowledge: One,
			}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}, opinion)
		}
	}
}
