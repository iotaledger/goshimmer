package ledgerstate

import (
	"math"
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
	unlockBlocks := transaction.UnlockBlocks()
	for i, input := range inputs {
		currentUnlockBlock := unlockBlocks[i]
		if currentUnlockBlock.Type() == ReferenceUnlockBlockType {
			currentUnlockBlock = unlockBlocks[unlockBlocks[i].(*ReferenceUnlockBlock).ReferencedIndex()]
		}

		unlockValid, unlockErr := input.UnlockValid(transaction, currentUnlockBlock)
		if !unlockValid || unlockErr != nil {
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
