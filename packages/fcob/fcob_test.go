package fcob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeriveOpinion(t *testing.T) {
	now := time.Now()

	// {t0, pending} -> {t1, Dislike, One}
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

	// {t0, Like, One} -> {t1, Dislike, One}
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

	// {t0, Like, Two} -> {t1, Dislike, Two}
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

	// {t0, Dislike, Two} -> {t1, Dislike, Pending}
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

	// {t0, Dislike, One} -> {t1, Dislike, One}
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

	//  {t0, Dislike, One}, {t1, Dislike, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            false,
				LevelOfKnowledge: One,
			}, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}}

			opinion := deriveOpinion(now.Add(10*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(10 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Like, One}, {t1, Dislike, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            true,
				LevelOfKnowledge: One,
			}, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}}

			opinion := deriveOpinion(now.Add(10*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(10 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Dislike, Two}, {t1, Like, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            false,
				LevelOfKnowledge: Two,
			}, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            true,
				LevelOfKnowledge: One,
			}}

			opinion := deriveOpinion(now.Add(6*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(6 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Dislike, Two}, {t1, Dislike, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            false,
				LevelOfKnowledge: Two,
			}, &Opinion{
				Timestamp:        now.Add(1 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}}

			opinion := deriveOpinion(now.Add(6*time.Second), conflictSet)
			assert.Equal(t, &Opinion{
				Timestamp:        now.Add(6 * time.Second),
				Liked:            false,
				LevelOfKnowledge: One,
			}, opinion)
		}
	}
}
