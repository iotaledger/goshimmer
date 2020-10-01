package ledgerstate

import (
	"fmt"

	"github.com/iotaledger/hive.go/stringify"
)

type ExtendableMerkleTree struct {
	leafCount      int
	root           string
	fullNodesStack []*ExtendableMerkleTreeNode
}

func (e *ExtendableMerkleTree) AddLeaf(value string) {
	currentNode := &ExtendableMerkleTreeNode{
		label: value,
		level: 0,
	}

	for len(e.fullNodesStack) >= 1 && currentNode.level == e.fullNodesStack[len(e.fullNodesStack)-1].level {
		currentNode = &ExtendableMerkleTreeNode{
			label: e.fullNodesStack[len(e.fullNodesStack)-1].label + currentNode.label,
			level: currentNode.level + 1,
		}

		e.fullNodesStack = e.fullNodesStack[:len(e.fullNodesStack)-1]
	}

	e.fullNodesStack = append(e.fullNodesStack, currentNode)

	fmt.Println(e.fullNodesStack)

	return
}

func (e *ExtendableMerkleTree) String() string {
	str := "ExtendableMerkleTree(\n"
	for _, node := range e.fullNodesStack {
		str += "    " + node.String() + ",\n"
	}
	str += ")"

	return str
}

type ExtendableMerkleTreeNode struct {
	label string
	level int
}

func (e *ExtendableMerkleTreeNode) String() string {
	return stringify.Struct("ExtendableMerkleTreeNode",
		stringify.StructField("label", e.label),
		stringify.StructField("level", e.level),
	)
}
