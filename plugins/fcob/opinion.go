package fcob

import (
	"github.com/iotaledger/goshimmer/packages/fpc"
)

type Opinion struct {
	like  fpc.Opinion
	final bool
}

func boolToOpinion(like bool) fpc.Opinion {
	if !like {
		return fpc.Dislike
	}
	return fpc.Like
}

func opinionToBool(opinion Opinion) bool {
	if opinion.like == fpc.Dislike {
		return false
	}
	return true
}

func (o Opinion) voted() bool {
	return o.final
}

func (o Opinion) liked() bool {
	return opinionToBool(o)
}
