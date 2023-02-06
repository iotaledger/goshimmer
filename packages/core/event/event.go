package event

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
)

//go:generate go run paramevents_generate.go

type /*typePrefix*/ Event /*paramCount*/ /*typedConstraints*/ struct {
	*event[func( /*paramTypes*/ )]
}

func New /*paramCount*/ /*typedConstraints*/ (opts ...Option) * /*typePrefix*/ Event /*untypedConstraints*/ {
	return & /*typePrefix*/ Event /*untypedConstraints*/ {
		event: newEvent[func( /*paramTypes*/ )](opts...),
	}
}

func (w * /*typePrefix*/ Event /*untypedConstraints*/) Trigger( /*typedParams*/ ) {
	if w.MaxTriggerCountReached() {
		return
	}

	w.hooks.ForEach(func(_ uint64, hook *Hook[func()]) bool {
		if hook.MaxTriggerCountReached() {
			hook.Unhook()

			return true
		}

		if workerPool := w.targetWorkerPool(hook); workerPool == nil {
			hook.trigger( /*untypedParams*/ )
		} else {
			workerPool.Submit(func() {
				hook.trigger( /*untypedParams*/ )
			})
		}

		return true
	})
}

func (w * /*typePrefix*/ Event /*untypedConstraints*/) LinkTo(optTarget ...* /*typePrefix*/ Event /*untypedConstraints*/) {
	w.linkTo(w.Trigger, lo.First(optTarget))
}
