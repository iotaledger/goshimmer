package server

import (
    "reflect"
)

var Events = &pluginEvents{
    AddNode:         &nodeIdEvent{make(map[uintptr]StringConsumer)},
    RemoveNode:      &nodeIdEvent{make(map[uintptr]StringConsumer)},
    ConnectNodes:    &nodeIdsEvent{make(map[uintptr]StringStringConsumer)},
    DisconnectNodes: &nodeIdsEvent{make(map[uintptr]StringStringConsumer)},
    NodeOnline:      &nodeIdEvent{make(map[uintptr]StringConsumer)},
    NodeOffline:     &nodeIdEvent{make(map[uintptr]StringConsumer)},
    Error:           &errorEvent{make(map[uintptr]ErrorConsumer)},
}

type pluginEvents struct {
    AddNode         *nodeIdEvent
    RemoveNode      *nodeIdEvent
    ConnectNodes    *nodeIdsEvent
    DisconnectNodes *nodeIdsEvent
    NodeOnline      *nodeIdEvent
    NodeOffline     *nodeIdEvent
    Error           *errorEvent
}

type nodeIdEvent struct {
    callbacks map[uintptr]StringConsumer
}

func (this *nodeIdEvent) Attach(callback StringConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *nodeIdEvent) Detach(callback StringConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *nodeIdEvent) Trigger(nodeId string) {
    for _, callback := range this.callbacks {
        callback(nodeId)
    }
}

type nodeIdsEvent struct {
    callbacks map[uintptr]StringStringConsumer
}

func (this *nodeIdsEvent) Attach(callback StringStringConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *nodeIdsEvent) Detach(callback StringStringConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *nodeIdsEvent) Trigger(sourceId string, targetId string) {
    for _, callback := range this.callbacks {
        callback(sourceId, targetId)
    }
}

type errorEvent struct {
    callbacks map[uintptr]ErrorConsumer
}

func (this *errorEvent) Attach(callback ErrorConsumer) {
    this.callbacks[reflect.ValueOf(callback).Pointer()] = callback
}

func (this *errorEvent) Detach(callback ErrorConsumer) {
    delete(this.callbacks, reflect.ValueOf(callback).Pointer())
}

func (this *errorEvent) Trigger(err error) {
    for _, callback := range this.callbacks {
        callback(err)
    }
}

type ErrorConsumer = func(err error)

type StringConsumer = func(str string)

type StringStringConsumer = func(sourceId string, targetId string)
