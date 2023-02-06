package generic

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/workerpool"
)

type base[TriggerFuncType any] struct {
	hooksCounter      uint64
	hooksCounterMutex sync.RWMutex

	link      *Hook[TriggerFuncType]
	linkMutex sync.Mutex

	hooks           *orderedmap.OrderedMap[uint64, *Hook[TriggerFuncType]]
	triggerCount    atomic.Uint64
	maxTriggerCount uint64
	workerPool      *workerpool.UnboundedWorkerPool
}

func newBase[TriggerFuncType any](opts ...Option) *base[TriggerFuncType] {
	b := &base[TriggerFuncType]{
		hooks: orderedmap.New[uint64, *Hook[TriggerFuncType]](),
	}

	for _, option := range opts {
		option(b)
	}

	return b
}

func (b *base[TriggerFuncType]) Hook(callback TriggerFuncType, opts ...Option) (hook *Hook[TriggerFuncType]) {
	hookID := b.nextHookID()

	hook = &Hook[TriggerFuncType]{
		callback: callback,
		unhook: func() {
			b.hooks.Delete(hookID)
		},
	}
	for _, option := range opts {
		option(hook)
	}

	b.hooks.Set(hookID, hook)

	return hook
}

func (b *base[TriggerFuncType]) WasTriggered() bool {
	return b.triggerCount.Load() > 0
}

func (b *base[TriggerFuncType]) TriggerCount() uint64 {
	return b.triggerCount.Load()
}

func (b *base[TriggerFuncType]) setMaxTriggerCount(maxTriggerCount uint64) {
	b.maxTriggerCount = maxTriggerCount
}

func (b *base[TriggerFuncType]) setWorkerPool(workerPool *workerpool.UnboundedWorkerPool) {
	b.setWorkerPool(workerPool)
}

func (b *base[TriggerFuncType]) shouldTrigger() bool {
	return b.triggerCount.Add(1) < b.maxTriggerCount || b.maxTriggerCount == 0
}

func (b *base[TriggerFuncType]) targetWorkerPool(hook *Hook[TriggerFuncType]) (workerPool *workerpool.UnboundedWorkerPool) {
	if hook.workerPool != nil {
		return hook.workerPool
	}

	return b.workerPool
}

func (b *base[TriggerFuncType]) nextHookID() uint64 {
	b.hooksCounterMutex.Lock()
	defer b.hooksCounterMutex.Unlock()

	hookID := b.hooksCounter
	b.hooksCounter++

	return hookID
}

func (b *base[TriggerFuncType]) linkTo(triggerFunc TriggerFuncType, target Hookable[TriggerFuncType]) {
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
