package vote

import (
	"errors"

	"github.com/iotaledger/hive.go/events"
)

var (
	// ErrVotingNotFound is returned when a voting for a given id wasn't found.
	ErrVotingNotFound = errors.New("no voting found")
)

// Voter votes on hashes.
type Voter interface {
	// Vote submits the given ID for voting with its initial Opinion.
	Vote(id string, initOpn Opinion) error
	// IntermediateOpinion gets intermediate Opinion about the given ID.
	IntermediateOpinion(id string) (Opinion, error)
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
	// Fired when internal errors occur.
	Error *events.Event
}

// OpinionCaller calls the given handler with an Opinion and its associated Id.
func OpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(id string, opinion Opinion))(params[0].(string), params[1].(Opinion))
}
