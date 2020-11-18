package statement

import (
	"sort"

	"github.com/iotaledger/goshimmer/packages/vote"
)

// Opinion holds the opinion at a specific round.
type Opinion struct {
	Value vote.Opinion
	Round uint8
}

// Opinions is a slice of Opinion.
type Opinions []Opinion

func (o Opinions) Len() int           { return len(o) }
func (o Opinions) Less(i, j int) bool { return o[i].Round < o[j].Round }
func (o Opinions) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }

// Last returns the opinion of the last round.
func (o Opinions) Last() Opinion {
	sort.Sort(o)
	return o[len(o)-1]
}

// Finalized returns true if the given opinion has been finalized.
func (o Opinions) Finalized(l int) bool {
	if len(o) < l {
		return false
	}
	sort.Sort(o)
	target := o[len(o)-1].Value
	for i := len(o) - 2; i >= l; i-- {
		if o[i].Value != target {
			return false
		}
	}
	return true
}
