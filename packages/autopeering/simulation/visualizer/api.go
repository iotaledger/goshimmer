package visualizer

import "fmt"

const (
	addNode uint32 = iota + 1
	removeNode
	addLink
	removeLink
	updateConvergence
	updateAvgNeighbors
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

func UpdateConvergence(c float64) {
	event := &Event{
		Type:   updateConvergence,
		Source: fmt.Sprintf("%v", c),
	}
	Writer(event)
}

func UpdateAvgNeighbors(avg float64) {
	event := &Event{
		Type:   updateAvgNeighbors,
		Source: fmt.Sprintf("%v", avg),
	}
	Writer(event)
}
