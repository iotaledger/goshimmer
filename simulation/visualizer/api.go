package visualizer

const (
	addNode uint32 = iota + 1
	removeNode
	addLink
	removeLink
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
