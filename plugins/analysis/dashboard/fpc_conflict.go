package dashboard

import "time"

// conflictSet is defined as a a map of conflict IDs and their conflict.
type conflictSet = map[string]conflict

// conflict defines the struct for the opinions of the nodes regarding a given conflict.
type conflict struct {
	NodesView map[string]voteContext `json:"nodesview" bson:"nodesview"`
	Modified  time.Time              `json:"modified" bson:"modified"`
}

type voteContext struct {
	NodeID   string  `json:"nodeid" bson:"nodeid"`
	Rounds   int     `json:"rounds" bson:"rounds"`
	Opinions []int32 `json:"opinions" bson:"opinions"`
	Outcome  int32   `json:"outcome" bson:"outcome"`
}

func newConflict() conflict {
	return conflict{
		NodesView: make(map[string]voteContext),
		Modified:  time.Now(),
	}
}

// isFinalized returns true if all the nodes have finalized a given conflict.
// It also returns false if the given conflict has an empty nodesView.
func (c conflict) isFinalized() bool {
	if len(c.NodesView) == 0 {
		return false
	}

	count := 0
	for _, context := range c.NodesView {
		if context.Outcome == liked || context.Outcome == disliked {
			count++
		}
	}

	return (count == len(c.NodesView))
}

// finalizedRatio returns the ratio of nodes that have finalized a given conflict.
func (c conflict) finalizedRatio() float64 {
	if len(c.NodesView) == 0 {
		return 0
	}
	count := 0
	for _, context := range c.NodesView {
		if context.Outcome == liked || context.Outcome == disliked {
			count++
		}
	}

	return (float64(count) / float64(len(c.NodesView)))
}

// isOlderThan returns true if the conflict is older (i.e., last modified time) than the given duration.
func (c conflict) isOlderThan(d time.Duration) bool {
	return c.Modified.Add(d).Before(time.Now())
}
