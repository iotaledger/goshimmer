package module

import (
	"github.com/iotaledger/hive.go/runtime/event"
)

type Interface interface {
	TriggerConstructed()
	WasConstructed() bool
	HookConstructed(func(), ...event.Option) *event.Hook[func()]

	TriggerInitialized()
	WasInitialized() bool
	HookInitialized(func(), ...event.Option) *event.Hook[func()]

	TriggerStopped()
	WasStopped() bool
	HookStopped(func(), ...event.Option) *event.Hook[func()]
}
