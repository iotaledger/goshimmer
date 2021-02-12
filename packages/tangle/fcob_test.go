package tangle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeriveOpinion(t *testing.T) {
	now := time.Now()

	// A{t0, pending} -> B{t-t0 < c, Dislike, One}
	{
		conflictSet := ConflictSet{
			OpinionEssence{
				timestamp:        now,
				liked:            false,
				levelOfKnowledge: Pending,
			}}

		opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
		assert.Equal(t, OpinionEssence{
			timestamp:        now.Add(1 * time.Second),
			liked:            false,
			levelOfKnowledge: One,
		}, opinion)
	}

	// A{t0, Like, One} -> B{c < t-t0 < c + d, Dislike, One}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            true,
					levelOfKnowledge: One,
				}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(1 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	// {t0, Like, Two} -> {t-t0 > c + d, Dislike, Two}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            true,
					levelOfKnowledge: Two,
				}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(1 * time.Second),
				liked:            false,
				levelOfKnowledge: Two,
			}, opinion)
		}
	}

	// A{t0, Dislike, Two}, B{t1, Dislike, Two} -> {t-t0 >> c + d, Dislike, Pending}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}, OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}}

			opinion := deriveOpinion(now.Add(1*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(1 * time.Second),
				liked:            false,
				levelOfKnowledge: Pending,
			}, opinion)
		}
	}

	// * double check this case
	//  {t0, Dislike, One}, {t1, Dislike, One} -> {t-t0 > 0, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: One,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            false,
					levelOfKnowledge: One,
				}}

			opinion := deriveOpinion(now.Add(10*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(10 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	// * double check this case
	//  {t0, Like, One}, {t1, Dislike, One} -> {t - t0 > c, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            true,
					levelOfKnowledge: One,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            false,
					levelOfKnowledge: One,
				}}

			opinion := deriveOpinion(now.Add(10*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(10 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Dislike, Two}, {t1, Like, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            true,
					levelOfKnowledge: One,
				}}

			opinion := deriveOpinion(now.Add(6*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(6 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}

	//  {t0, Dislike, Two}, {t1, Dislike, One} -> {t10, Dislike, one}
	{
		{
			conflictSet := ConflictSet{
				OpinionEssence{
					timestamp:        now,
					liked:            false,
					levelOfKnowledge: Two,
				}, OpinionEssence{
					timestamp:        now.Add(1 * time.Second),
					liked:            false,
					levelOfKnowledge: One,
				}}

			opinion := deriveOpinion(now.Add(6*time.Second), conflictSet)
			assert.Equal(t, OpinionEssence{
				timestamp:        now.Add(6 * time.Second),
				liked:            false,
				levelOfKnowledge: One,
			}, opinion)
		}
	}
}
