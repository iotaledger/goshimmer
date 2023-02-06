package event

//
// import (
// 	"github.com/iotaledger/hive.go/core/generics/options"
// )
//
// type With2Params[A, B any] struct {
// 	*base
// }
//
// func New2[A, B any]() *With2Params[A, B] {
// 	return &With2Params[A, B]{
// 		base: newBase(),
// 	}
// }
//
// func (w *With2Params[A, B]) Hook(callback func(a A, b B), opts ...options.Option[Hook]) (hook *Hook) {
// 	return w.base.hook(callback, opts...)
// }
//
// func (w *With2Params[A, B]) Trigger(a A, b B) {
// 	if w.triggerCount.Inc() >= w.maxTriggerCount && w.maxTriggerCount > 0 {
// 		return
// 	}
//
// 	w.hooks.ForEach(func(_ uint64, hook *Hook) bool {
// 		if hook.triggerCount.Inc() >= hook.maxTriggerCount && hook.maxTriggerCount > 0 {
// 			return true
// 		}
//
// 		if hook.workerPool == nil {
// 			hook.callback.(func(A, B))(a, b)
//
// 			return true
// 		}
//
// 		hook.workerPool.Submit(func() {
// 			hook.callback.(func(A, B))(a, b)
// 		})
//
// 		return true
// 	})
// }
