package vote

import (
	"context"
)

// OpinionGiver gives opinions about the given IDs.
type OpinionGiver interface {
	// Query queries the OpinionGiver for its opinions on the given IDs.
	// The passed in context can be used to signal cancellation of the query.
	Query(ctx context.Context, ids []string) (Opinions, error)
}

type OpinionQueryFunc func(ctx context.Context, ids []string) (Opinions, error)

func (oqf OpinionQueryFunc) Query(ctx context.Context, ids []string) (Opinions, error) {
	return oqf(ctx, ids)
}

// OpinionGiverFunc is a function which gives a slice of OpinionGivers or an error.
type OpinionGiverFunc func() ([]OpinionGiver, error)

// Opinions is a slice of Opinion.
type Opinions []Opinion

// Opinion is an opinion about a given thing.
type Opinion byte

const (
	Like    Opinion = 1 << 0
	Dislike Opinion = 1 << 1
	Unknown Opinion = 1 << 2
)

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
