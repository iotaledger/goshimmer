package node

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/runtime/event"
)

var Events *NodeEvents

type NodeEvents struct {
	AddPlugin *event.Event1[*AddEvent]
}

func newNodeEvents() *NodeEvents {
	return &NodeEvents{
		AddPlugin: event.New1[*AddEvent](),
	}
}

type AddEvent struct {
	Name   string
	Status int
}

func init() {
	Events = newNodeEvents()
}

type PluginEvents struct {
	Init      *event.Event1[*InitEvent]
	Configure *event.Event1[*ConfigureEvent]
	Run       *event.Event1[*RunEvent]
}

func newPluginEvents() *PluginEvents {
	return &PluginEvents{
		Init:      event.New1[*InitEvent](),
		Configure: event.New1[*ConfigureEvent](),
		Run:       event.New1[*RunEvent](),
	}
}

type InitEvent struct {
	Plugin    *Plugin
	Container *dig.Container
}

type ConfigureEvent struct {
	Plugin *Plugin
}

type RunEvent struct {
	Plugin *Plugin
}
