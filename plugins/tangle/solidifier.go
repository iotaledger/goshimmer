package tangle

import (
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/ternary"
)

func UpdateSolidity(transaction *Transaction) (bool, errors.IdentifiableError) {
    // abort if transaction is solid already
    txMetadata, err := transaction.GetMetaData()
    if err != nil {
        return false, err
    } else if txMetadata.GetSolid() {
        return true, nil
    }

    // check solidity of branch transaction if it is not genesis
    if branchTransactionHash := transaction.GetBranchTransactionHash(); branchTransactionHash != ternary.Trinary("999999") {
        // abort if branch transaction is missing
        branchTransaction, err := GetTransaction(branchTransactionHash)
        if err != nil {
            return false, err
        } else if branchTransaction == nil {
            return false, nil
        }

        // abort if branch transaction is not solid
        if branchTransactionMetadata, err := branchTransaction.GetMetaData(); err != nil {
            return false, err
        } else if !branchTransactionMetadata.GetSolid() {
            return false, nil
        }
    }

    // check solidity of branch transaction if it is not genesis
    if trunkTransactionHash := transaction.GetBranchTransactionHash(); trunkTransactionHash != ternary.Trinary("999999") {
        // abort if trunk transaction is missing
        trunkTransaction, err := GetTransaction(trunkTransactionHash)
        if err != nil {
            return false, err
        } else if trunkTransaction == nil {
            return false, nil
        }

        // abort if trunk transaction is not solid
        if trunkTransactionMetadata, err := trunkTransaction.GetMetaData(); err != nil {
            return false, err
        } else if !trunkTransactionMetadata.GetSolid() {
            return false, nil
        }
    }

    // propagate solidity to all approvers
    txMetadata.SetSolid(true)

    return true, nil
}