package ledgerstate

import (
	"math"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/types"
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

// AliasInitialStateValid is an internal utility function that checks if aliases are created by the transaction with
// valid initial states.
// Initial state of an alias is valid, if and only if:
//  - there is no "chained" alias with the same ID on the input side
//  - the alias itself has the origin flag set, and state index is 0.
func AliasInitialStateValid(inputs Outputs, transaction *Transaction) bool {
	// are there any aliases present on the output side?
	outputAliases := make(map[AliasAddress]*AliasOutput)
	for _, output := range transaction.Essence().Outputs() {
		if output.Type() != AliasOutputType {
			continue
		}
		alias, ok := output.(*AliasOutput)
		if !ok {
			// alias output can't be casted to its type, fail the validation
			return false
		}
		if _, exists := outputAliases[*alias.GetAliasAddress()]; exists {
			// duplicated alias found on output side, not valid, there can only ever be one output with a given AliasAddress
			return false
		}
		outputAliases[*alias.GetAliasAddress()] = alias
	}
	if len(outputAliases) == 0 {
		// there are no aliases on the output side, check is valid
		return true
	}
	// gather what aliases are present on the input side
	inputAliases := make(map[AliasAddress]types.Empty)
	for _, input := range inputs {
		if input.Type() != AliasOutputType {
			continue
		}
		alias, ok := input.(*AliasOutput)
		if !ok {
			// alias output can't be casted to its type, fail the validation
			return false
		}
		if _, exists := inputAliases[*alias.GetAliasAddress()]; exists {
			// duplicated alias found on input side (this should never happen tough!)
			return false
		}
		inputAliases[*alias.GetAliasAddress()] = types.Empty{}
	}

	// now comes the initial state validation
	for addy, output := range outputAliases {
		if _, exists := inputAliases[addy]; exists {
			// output alias is present on input side, transition rules enforced by the unlocked input alias
			// pair found, remove from "usable" input aliases
			delete(inputAliases, addy)
			continue
		}
		// alias output has no pair on the input side, so it must be an origin alias
		// an origin alias must have the alias address bytes zeroed out and state index must be 0, furthermore the
		// origin flag must be set.
		// note, that we have to access the raw bytes, because GetAliasAddress() automatically returns the calculated
		// address bytes, which will be set after booking, when storing the output.
		if !output.IsOrigin() || !output.aliasAddress.IsNil() || output.GetStateIndex() != 0 {
			// either the alias address bytes are not zero, or state index
			return false
		}
	}
	return true
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
func (g *UnlockGraph) dfs(node uint16, visited, recursionStack []bool) bool {
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
