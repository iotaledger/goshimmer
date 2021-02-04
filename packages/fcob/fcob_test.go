package fcob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeriveOpinion(t *testing.T) {
	now := time.Now()

	// A{t0, pending} -> B{t-t0 < c, Dislike, One}
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

	// A{t0, Like, One} -> B{c < t-t0 < c + d, Dislike, One}
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

	// {t0, Like, Two} -> {t-t0 > c + d, Dislike, Two}
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

	// A{t0, Dislike, Two}, B{t1, Dislike, Two} -> {t-t0 >> c + d, Dislike, Pending}
	{
		{
			conflictSet := ConflictSet{&Opinion{
				Timestamp:        now,
				Liked:            false,
				LevelOfKnowledge: Two,
			}, &Opinion{
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

	// * double check this case
	//  {t0, Dislike, One}, {t1, Dislike, One} -> {t-t0 > 0, Dislike, one}
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

	// * double check this case
	//  {t0, Like, One}, {t1, Dislike, One} -> {t - t0 > c, Dislike, one}
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
