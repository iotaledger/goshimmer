package event

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/workerpool"
)

type event[TriggerFunc any] struct {
	hooks        *orderedmap.OrderedMap[uint64, *Hook[TriggerFunc]]
	hooksCounter atomic.Uint64
	link         *Hook[TriggerFunc]
	linkMutex    sync.Mutex

	*triggerSettings
}

func newEvent[TriggerFunc any](opts ...Option) *event[TriggerFunc] {
	b := &event[TriggerFunc]{
		hooks:           orderedmap.New[uint64, *Hook[TriggerFunc]](),
		triggerSettings: options.Apply(new(triggerSettings), opts),
	}

	for _, option := range opts {
		option(b.triggerSettings)
	}

	return b
}

func (b *event[TriggerFunc]) Hook(triggerFunc TriggerFunc, opts ...Option) (hook *Hook[TriggerFunc]) {
	hookID := b.hooksCounter.Add(1)
	hook = newHook(triggerFunc, func() { b.hooks.Delete(hookID) }, opts...)

	b.hooks.Set(hookID, hook)

	return hook
}

func (b *event[TriggerFunc]) linkTo(triggerFunc TriggerFunc, target hookable[TriggerFunc]) {
	b.linkMutex.Lock()
	defer b.linkMutex.Unlock()

	if b.link != nil {
		b.link.Unhook()
	}

	if target == nil {
		b.link = nil
	} else {
		b.link = target.Hook(triggerFunc)
	}
}

func (b *event[TriggerFunc]) targetWorkerPool(hook *Hook[TriggerFunc]) (workerPool *workerpool.UnboundedWorkerPool) {
	if hook.workerPool != nil {
		return hook.workerPool
	}

	return b.workerPool
}

type hookable[TriggerFunc any] interface {
	Hook(callback TriggerFunc, opts ...Option) (hook *Hook[TriggerFunc])
}
