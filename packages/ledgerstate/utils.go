package ledgerstate

import (
	"math"

	"github.com/cockroachdb/errors"
)

// TransactionBalancesValid is an internal utility function that checks if the sum of the balance changes equals to 0.
func TransactionBalancesValid(inputs Outputs, outputs Outputs) (valid bool) {
	consumedCoins := make(map[Color]uint64)
	for _, input := range inputs {
		input.Balances().ForEach(func(color Color, balance uint64) bool {
			consumedCoins[color], valid = SafeAddUint64(consumedCoins[color], balance)

			return valid
		})

		if !valid {
			return
		}
	}

	recoloredCoins := uint64(0)
	for _, output := range outputs {
		output.Balances().ForEach(func(color Color, balance uint64) bool {
			switch color {
			case ColorIOTA, ColorMint:
				recoloredCoins, valid = SafeAddUint64(recoloredCoins, balance)
			default:
				consumedCoins[color], valid = SafeSubUint64(consumedCoins[color], balance)
			}

			return valid
		})

		if !valid {
			return
		}
	}

	unspentCoins := uint64(0)
	for _, remainingBalance := range consumedCoins {
		if unspentCoins, valid = SafeAddUint64(unspentCoins, remainingBalance); !valid {
			return
		}
	}

	return unspentCoins == recoloredCoins
}

// UnlockBlocksValid is an internal utility function that checks if the UnlockBlocks are matching the referenced Inputs.
func UnlockBlocksValid(inputs Outputs, transaction *Transaction) (valid bool) {
	unlockValid, unlockErr := UnlockBlocksValidWithError(inputs, transaction)

	return unlockValid && unlockErr == nil
}

// UnlockBlocksValidWithError is an internal utility function that checks if the UnlockBlocks are matching the referenced Inputs.
// In case an unlockblock is invalid, it returns the error that caused it.
func UnlockBlocksValidWithError(inputs Outputs, transaction *Transaction) (bool, error) {
	unlockBlocks := transaction.UnlockBlocks()
	cyclePresent, err := checkReferenceCycle(unlockBlocks)
	if err != nil {
		return false, errors.Errorf("unlock blocks are semantically invalid: %w", err)
	}
	if cyclePresent {
		return false, errors.New("unlock blocks contain cyclic dependency, no signature present for an unlock path")
	}
	for i, input := range inputs {
		currentUnlockBlock := unlockBlocks[i]
		if currentUnlockBlock.Type() == ReferenceUnlockBlockType {
			currentUnlockBlock = unlockBlocks[unlockBlocks[i].(*ReferenceUnlockBlock).ReferencedIndex()]
		}

		unlockValid, unlockErr := input.UnlockValid(transaction, currentUnlockBlock, inputs)
		if !unlockValid || unlockErr != nil {
			return false, unlockErr
		}
	}

	return true, nil
}

// SafeAddUint64 adds two uint64 values. It returns the result and a valid flag that indicates whether the addition is
// valid without causing an overflow.
func SafeAddUint64(a uint64, b uint64) (result uint64, valid bool) {
	valid = math.MaxUint64-a >= b
	result = a + b
	return
}

// SafeSubUint64 subtracts two uint64 values. It returns the result and a valid flag that indicates whether the
// subtraction is valid without causing an overflow.
func SafeSubUint64(a uint64, b uint64) (result uint64, valid bool) {
	valid = b <= a
	result = a - b
	return
}

// checkReferenceCycle builds a graph from the unlock block references and detects circular referencing. It returns an error
// if unlock block references are semantically invalid.
func checkReferenceCycle(blocks UnlockBlocks) (bool, error) {
	// build graph
	unlockGraph, err := NewUnlockGraph(blocks)
	if err != nil {
		return false, err
	}
	// check for cycle
	return unlockGraph.IsCycleDetected(), nil
}

// UnlockGraph builds a graph from the references of the unlock blocks with the aim of detecting cycles.
type UnlockGraph struct {
	// Vertices are the unlock blocks themselves, identified by their index
	// uint16 is enough
	Vertices []uint16
	// maps unlockBlock to referenced unlocked block
	// each vertex has at most 1 referenced unlock block, so one outgoing edge
	Edges map[uint16]uint16
}

// NewUnlockGraph creates a new UnlockGraph and checks semantic validity of the unlock block references.
func NewUnlockGraph(blocks UnlockBlocks) (*UnlockGraph, error) {
	if len(blocks) > MaxInputCount {
		return nil, errors.Errorf("number of unlock blocks %d exceeds max input count %d", len(blocks), MaxInputCount)
	}
	g := &UnlockGraph{
		Vertices: make([]uint16, len(blocks)),
		Edges:    make(map[uint16]uint16, len(blocks)),
	}
	for i, block := range blocks {
		g.Vertices[i] = uint16(i)
		switch block.Type() {
		case SignatureUnlockBlockType:
			// no adjacent vertex as a SignatureUnlockBlockType can't reference an other one
		case ReferenceUnlockBlockType:
			// a reference unlock block can not point to another reference unlock block
			refIndex := block.(*ReferenceUnlockBlock).ReferencedIndex()
			if blocks[refIndex].Type() == ReferenceUnlockBlockType {
				return nil, errors.Errorf("reference unlock block %d points to another reference unlock block %d", i, refIndex)
			}
			g.Edges[uint16(i)] = refIndex
		case AliasUnlockBlockType:
			g.Edges[uint16(i)] = block.(*AliasUnlockBlock).AliasInputIndex()
		default:
			return nil, errors.Errorf("unknown unlock block type at index %d", i)
		}
	}
	return g, nil
}

// IsCycleDetected checks if a cycle is detected by checking the dfs paths from every node.
func (g *UnlockGraph) IsCycleDetected() bool {
	visited := make([]bool, len(g.Vertices))
	recursionStack := make([]bool, len(g.Vertices))

	for _, node := range g.Vertices {
		if !visited[node] {
			// visit node
			if g.dfs(node, visited, recursionStack) {
				return true
			}
		}
	}
	return false
}

// dfs does a depth-first traversal and looks for back edges (cycles).
func (g *UnlockGraph) dfs(node uint16, visited []bool, recursionStack []bool) bool {
	// mark current node visited
	visited[node] = true
	// add to recursion stack
	recursionStack[node] = true

	// recursively visit path from node
	referenced, hasReferenced := g.Edges[node]
	if hasReferenced {
		if !visited[referenced] {
			if g.dfs(referenced, visited, recursionStack) {
				return true
			}
		} else if recursionStack[referenced] {
			// we arrived at a cycle
			return true
		}
	}

	// pop node from recursion stack before return (backtracking)
	recursionStack[node] = false
	return false
}
