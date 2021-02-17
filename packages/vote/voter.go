package vote

import (
	"errors"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
)

var (
	// ErrVotingNotFound is returned when a voting for a given id wasn't found.
	ErrVotingNotFound = errors.New("no voting found")
)

// Voter votes on hashes.
type Voter interface {
	// Vote submits the given ID for voting with its initial Opinion.
	Vote(id string, objectType ObjectType, initOpn opinion.Opinion) error
	// IntermediateOpinion gets intermediate Opinion about the given ID.
	IntermediateOpinion(id string) (opinion.Opinion, error)
	// Events returns the Events instance of the given Voter.
	Events() Events
}

// DRNGRoundBasedVoter is a Voter which votes in rounds and uses random numbers which
// were generated in a decentralized fashion.
type DRNGRoundBasedVoter interface {
	Voter
	// Round starts a new round.
	Round(rand float64) error
}

// Events defines events which happen on a Voter.
type Events struct {
	// Fired when an Opinion has been finalized.
	Finalized *events.Event
	// Fired when an Opinion couldn't be finalized.
	Failed *events.Event
	// Fired when a DRNGRoundBasedVoter has executed a round.
	RoundExecuted *events.Event
	// Fired when internal errors occur.
	Error *events.Event
}

// RoundStats encapsulates data about an executed round.
type RoundStats struct {
	// The time it took to complete a round.
	Duration time.Duration `json:"duration"`
	// The rand number used during the round.
	RandUsed float64 `json:"rand_used"`
	// The vote contexts on which opinions were formed and queried.
	// This list does not include the vote contexts which were finalized/aborted
	// during the execution of the round.
	// Create a copy of this map if you need to modify any of its elements.
	ActiveVoteContexts map[string]*Context `json:"active_vote_contexts"`
	// The opinions which were queried during the round per opinion giver.
	QueriedOpinions []opinion.QueriedOpinions `json:"queried_opinions"`
}

// OpinionEvent is the struct containing data to be passed around with Finalized and Failed events.
type OpinionEvent struct {
	// ID is the of the conflict.
	ID string
	// Opinion is an opinion about a conflict.
	Opinion opinion.Opinion
	// Ctx contains all relevant infos regarding the conflict.
	Ctx Context
}

// OpinionCaller calls the given handler with an OpinionEvent (containing its opinions, its associated ID and context).
func OpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *OpinionEvent))(params[0].(*OpinionEvent))
}

// RoundStatsCaller calls the given handler with a RoundStats.
func RoundStatsCaller(handler interface{}, params ...interface{}) {
	handler.(func(stats *RoundStats))(params[0].(*RoundStats))
}
