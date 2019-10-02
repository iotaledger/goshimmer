package visualizer

import "fmt"

const (
	addNode uint32 = iota + 1
	removeNode
	addLink
	removeLink
	updateDegree
)

func AddNode(id string) {
	event := &Event{
		Type:   addNode,
		Source: id,
		Dest:   "",
	}
	Writer(event)
}

func RemoveNode(id string) {
	event := &Event{
		Type:   removeNode,
		Source: id,
		Dest:   "",
	}
	Writer(event)
}

func AddLink(src, dest string) {
	event := &Event{
		Type:   addLink,
		Source: src,
		Dest:   dest,
	}
	Writer(event)
}

func RemoveLink(src, dest string) {
	event := &Event{
		Type:   removeLink,
		Source: src,
		Dest:   dest,
	}
	Writer(event)
}

func UpdateDegree(degree int) {
	event := &Event{
		Type:   updateDegree,
		Source: fmt.Sprintf("%v", degree),
	}
	Writer(event)
}
