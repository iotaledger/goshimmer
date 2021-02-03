package opinion

import (
	"context"

	"github.com/iotaledger/hive.go/identity"
)

// OpinionGiver gives opinions about the given IDs.
type OpinionGiver interface {
	// Query queries the OpinionGiver for its opinions on the given IDs.
	// The passed in context can be used to signal cancellation of the query.
	Query(ctx context.Context, conflictIDs []string, timestampIDs []string) (Opinions, error)
	// ID returns the ID of the opinion giver.
	ID() identity.ID
}

// QueriedOpinions represents queried opinions from a given opinion giver.
type QueriedOpinions struct {
	// The ID of the opinion giver.
	OpinionGiverID string `json:"opinion_giver_id"`
	// The map of IDs to opinions.
	Opinions map[string]Opinion `json:"opinions"`
	// The amount of times the opinion giver's opinion has counted.
	// Usually this number is 1 but due to randomization of the queried opinion givers,
	// the same opinion giver's opinions might be taken into account multiple times.
	TimesCounted int `json:"times_counted"`
}

// OpinionGiverFunc is a function which gives a slice of OpinionGivers or an error.
type OpinionGiverFunc func() ([]OpinionGiver, error)

// Opinions is a slice of Opinion.
type Opinions []Opinion

// Opinion is an opinion about a given thing.
type Opinion byte

const (
	// Like defines a Like opinion.
	Like Opinion = 1 << 0
	// Dislike defines a Dislike opinion.
	Dislike Opinion = 1 << 1
	// Unknown defines an unknown opinion.
	Unknown Opinion = 1 << 2
)

func (o Opinion) String() string {
	switch {
	case o == Like:
		return "Like"
	case o == Dislike:
		return "Dislike"
	}
	return "Unknown"
}

// ConvertInt32Opinion converts the given int32 to an Opinion.
func ConvertInt32Opinion(x int32) Opinion {
	switch {
	case x == 1<<0:
		return Like
	case x == 1<<1:
		return Dislike
	}
	return Unknown
}

// ConvertInts32ToOpinions converts the given slice of int32 to a slice of Opinion.
func ConvertInts32ToOpinions(opinions []int32) []Opinion {
	result := make([]Opinion, len(opinions))
	for i, opinion := range opinions {
		result[i] = ConvertInt32Opinion(opinion)
	}
	return result
}

// ConvertOpinionToInt32 converts the given Opinion to an int32.
func ConvertOpinionToInt32(x Opinion) int32 {
	switch {
	case x == Like:
		return 1
	case x == Dislike:
		return 2
	}
	return 4
}

// ConvertOpinionsToInts32 converts the given slice of Opinion to a slice of int32.
func ConvertOpinionsToInts32(opinions []Opinion) []int32 {
	result := make([]int32, len(opinions))
	for i, opinion := range opinions {
		result[i] = ConvertOpinionToInt32(opinion)
	}
	return result
}
