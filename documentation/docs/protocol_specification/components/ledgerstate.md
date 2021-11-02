---
description: The unspent transaction output (UTXO) model defines a ledger state where balances are not directly associated with addresses but with the outputs of transactions. Transactions specify the outputs of previous transactions as inputs, which are consumed in order to create new outputs.
image: /img/protocol_specification/utxo_fund_flow.png
keywords:
- transactions
- ledger state
- unlock block
- essence
- utxo
- input
- signature unlock block
- reference unlock block
- conflict branch
- aggregate branch
---
## UTXO model

The unspent transaction output (UTXO) model defines a ledger state where balances are not directly associated with addresses but with the outputs of transactions. In this model, transactions specify the outputs of previous transactions as inputs, which are consumed in order to create new outputs. 
A transaction must consume the entirety of the specified inputs. The section unlocking the inputs is called an *unlock block*. An unlock block may contain a signature proving ownership of a given input's address and/or other unlock criteria.

The following image depicts the flow of funds using UTXO:

[![Flow of funds using UTXO](/img/protocol_specification/utxo_fund_flow.png "Flow of funds using UTXO")](/img/protocol_specification/utxo_fund_flow.png )

## Transaction Layout

A _Transaction_ payload is made up of two parts:
1. The _Transaction Essence_ part contains: version, timestamp, nodeID of the aMana pledge, nodeID of the cMana pledge, inputs, outputs and an optional data payload.
2. The _Unlock Blocks_ which unlock the _Transaction Essence_'s inputs. In case the unlock block contains a signature, it signs the entire _Transaction Essence_ part.

All values are serialized in little-endian encoding (it stores the most significant byte of a word at the largest address and the smallest byte at the smallest address). The serialized form of the transaction is deterministic, meaning the same logical transaction always results in the same serialized byte sequence.

### Transaction Essence

The _Transaction Essence_ of a _Transaction_ carries a version, timestamp, nodeID of the aMana pledge, nodeID of the cMana pledge, inputs, outputs and an optional data payload.

### Inputs

The _Inputs_ part holds the inputs to consume, that in turn fund the outputs of the _Transaction Essence_. There is only one supported type of input as of now, the _UTXO Input_. In the future, more types of inputs may be specified as part of protocol upgrades.

Each defined input must be accompanied by a corresponding _Unlock Block_ at the same index in the _Unlock Blocks_ part of the _Transaction_. 
If multiple inputs may be unlocked through the same _Unlock Block_, the given _Unlock Block_ only needs to be specified at the index of the first input that gets unlocked by it. 
Subsequent inputs that are unlocked through the same data must have a _Reference Unlock Block_ pointing to the previous _Unlock Block_. 
This ensures that no duplicate data needs to occur in the same transaction.

#### UTXO Input

| Name                     | Type          | Description                                                             |
| ------------------------ | ------------- | ----------------------------------------------------------------------- |
| Input Type               | uint8         | Set to value 0 to denote an _UTXO Input_.                               |
| Transaction ID           | ByteArray[32] | The BLAKE2b-256 hash of the transaction from which the UTXO comes from. |
| Transaction Output Index | uint16        | The index of the output on the referenced transaction to consume.       |



A _UTXO Input_ is an input which references an output of a previous transaction by using the given transaction's BLAKE2b-256 hash + the index of the output on that transaction. 
A _UTXO Input_ must be accompanied by an _Unlock Block_ for the corresponding type of output the _UTXO Input_ is referencing.

Example: If the input references outputs to an Ed25519 address, then the corresponding unlock block must be of type _Signature Unlock Block_ holding an Ed25519 signature.

### Outputs

The _Outputs_ part holds the outputs to create with this _Transaction Payload_. There are different types of output: 
* _SigLockedSingleOutput_
* _SigLockedAssetOutput_

#### SigLockedSingleOutput

| Name            | Type                                                               | Description                                                                                         |
| --------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| Output Type     | uint8                                                              | Set to value 0 to denote a _SigLockedSingleOutput_.                                                 |
| Address `oneOf` | [Ed25519 Address](#ed25519-address) \| [BLS Address](#bls-address) | The raw bytes of the Ed25519/BLS address which is a BLAKE2b-256 hash of the Ed25519/BLS  public key |
| Balance         | uint64                                                             | The balance of IOTA tokens to deposit with this _SigLockedSingleOutput_ output.                     |


##### Ed25519 Address

| Name         | Type          | Description                                                                                 |
| ------------ | ------------- | ------------------------------------------------------------------------------------------- |
| Address Type | uint8         | Set to value 0 to denote an _Ed25519 Address_.                                              |
| Address      | ByteArray[32] | The raw bytes of the Ed25519 address which is a BLAKE2b-256 hash of the Ed25519 public key. |


#### BLS Address

| Name         | Type          | Description                                                                         |
| ------------ | ------------- | ----------------------------------------------------------------------------------- |
| Address Type | uint8         | Set to value 1 to denote a _BLS Address_.                                           |
| Address      | ByteArray[49] | The raw bytes of the BLS address which is a BLAKE2b-256 hash of the BLS public key. |


The _SigLockedSingleOutput_ defines an output holding an IOTA balance linked to a single address; it is unlocked via a valid signature proving ownership over the given address. Such output may hold an address of different types.

#### SigLockedAssetOutput

| Name                 | Type                                                               | Description                                                                                         |
| -------------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| Output Type          | uint8                                                              | Set to value 1 to denote a _SigLockedAssetOutput_.                                                  |
| Address `oneOf`      | [Ed25519 Address](#ed25519-address) \| [BLS Address](#bls-address) | The raw bytes of the Ed25519/BLS address which is a BLAKE2b-256 hash of the Ed25519/BLS  public key |
| Balances count       | uint32                                                             | The number of individual balances.                                                                  |
| AssetBalance `anyOf` | [Asset Balance](#asset-balance)                                    | The balance of the tokenized asset.                                                                 |


##### Asset Balance

The balance of the tokenized asset.

| Name    | Type          | Description                         |
| ------- | ------------- | ----------------------------------- |
| AssetID | ByteArray[32] | The ID of the tokenized asset       |
| Balance | uint64        | The balance of the tokenized asset. |

The _SigLockedAssetOutput_ defines an output holding a balance for each specified tokenized asset linked to a single address; it is unlocked via a valid signature proving ownership over the given address. Such output may hold an address of different types.
The ID of any tokenized asset is defined by the BLAKE2b-256 hash of the OutputID that created the asset.

### Payload

The payload part of a _Transaction Essence_ may hold an optional payload. This payload does not affect the validity of the _Transaction Essence_. If the transaction is not valid, then the payload *shall* be discarded.

### Unlock Blocks

The _Unlock Blocks_ part holds the unlock blocks unlocking inputs within a _Transaction Essence_.

There are different types of _Unlock Blocks_:
| Name                   | Unlock Type | Description                                                                                                                                   |
| ---------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Signature Unlock Block | 0           | An unlock block holding one or more signatures unlocking one or more inputs.                                                                  |
| Reference Unlock Block | 1           | An unlock block which must reference a previous unlock block which unlocks also the input at the same index as this _Reference Unlock Block_. |

#### Signature Unlock Block

| Name              | Type                                                               | Description                                                                                         |
| ----------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------- |
| Unlock Type       | uint8                                                              | Set to value 0 to denote a _Signature Unlock Block_.                                                |
| Signature `oneOf` | [Ed25519 Address](#ed25519-address) \| [BLS Address](#bls-address) | The raw bytes of the Ed25519/BLS address which is a BLAKE2b-256 hash of the Ed25519/BLS  public key |


A _Signature Unlock Block_ defines an _Unlock Block_ which holds one or more signatures unlocking one or more inputs.
Such a block signs the entire _Transaction Essence_ part of a _Transaction Payload_ including the optional payload.

#### Reference Unlock block

| Name        | Type   | Description                                          |
| ----------- | ------ | ---------------------------------------------------- |
| Unlock Type | uint8  | Set to value 1 to denote a _Reference Unlock Block_. |
| Reference   | uint16 | Represents the index of a previous unlock block.     |


A _Reference Unlock Block_ defines an _Unlock Block_ that references a previous _Unlock Block_ (that must not be another _Reference Unlock Block_). It must be used if multiple inputs can be unlocked through the same origin _Unlock Block_.

Example:
Consider a _Transaction Essence_ containing _UTXO Inputs_ A, B and C, where A and C are both spending the UTXOs originating from the same Ed25519 address. The _Unlock Block_ part must thereby have the following structure:

| Index | Must Contain                                                                                               |
| ----- | ---------------------------------------------------------------------------------------------------------- |
| 0     | A _Signature Unlock Block_ holding the corresponding Ed25519 signature to unlock A and C.                  |
| 1     | A _Signature Unlock Block_ that unlocks B.                                                                 |
| 2     | A _Reference Unlock Block_ that references index 0, since C also gets unlocked by the same signature as A. |

## Validation

A _Transaction_ payload has different validation stages since some validation steps can only be executed at the point when certain information has (or has not) been received. We, therefore, distinguish between syntactical and semantic validation.

### Transaction Syntactical Validation

This validation can commence as soon as the transaction data has been received in its entirety. It validates the structure but not the signatures of the transaction. A transaction must be discarded right away if it does not pass this stage.

The following criteria define whether the transaction passes the syntactical validation:
* Transaction Essence:
    * `Transaction Essence Version` value must be 0.
    * The `timestamp` of the _Transaction Essence_ must be older than (or equal to) the `timestamp` of the message
      containing the transaction by at most 10 minutes.
    * A _Transaction Essence_ must contain at least one input and output.
* Inputs:
    * `Inputs Count` must be 0 < x < 128.
    * At least one input must be specified.
    * `Input Type` value must be 0, denoting an `UTXO Input`.
    * `UTXO Input`:
        * `Transaction Output Index` must be 0 ≤ x < 128.
        * Every combination of `Transaction ID` + `Transaction Output Index` must be unique in the inputs set.
    * Inputs must be in lexicographical order of their serialized form.<sup>1</sup>
* Outputs:
    * `Outputs Count` must be 0 < x < 128.
    * At least one output must be specified.
    * `Output Type` must be 0, denoting a `SigLockedSingleOutput`.
    * `SigLockedSingleOutput`:
        * `Address Type` must either be 0 or 1, denoting an `Ed25519` - or `BLS` address .
        * The `Address` must be unique in the set of `SigLockedSingleOutputs`.
        * `Amount` must be > 0.
    * Outputs must be in lexicographical order by their serialized form. This ensures that serialization of the transaction becomes deterministic, meaning that libraries always produce the same bytes given the logical transaction.
    * Accumulated output balance must not exceed the total supply of tokens `2,779,530,283,277,761`.
* `Payload Length` must be 0 (to indicate that there's no payload) or be valid for the specified payload type.
* `Payload Type` must be one of the supported payload types if `Payload Length` is not 0.
* `Unlock Blocks Count` must match the number of inputs. Must be 0 < x < 128.
* `Unlock Block Type` must either be 0 or 1, denoting a `Signature Unlock Block` or `Reference Unlock block`.
* `Signature Unlock Blocks` must define either an `Ed25519`- or `BLS Signature`.
* A `Signature Unlock Block` unlocking multiple inputs must only appear once (be unique) and be positioned at the same index of the first input it unlocks. All other inputs unlocked by the same `Signature Unlock Block` must have a companion `Reference Unlock Block` at the same index as the corresponding input that points to the origin `Signature Unlock Block`.
* `Reference Unlock Blocks` must specify a previous `Unlock Block` that is not of type `Reference Unlock Block`. The referenced index must therefore be smaller than the index of the `Reference Unlock Block`.
* Given the type and length information, the _Transaction_ must consume the entire byte array the `Payload Length` field in the _Message_ defines.


### Transaction Semantic Validation

The following criteria define whether the transaction passes the semantic validation:
1. All the UTXOs the transaction references are known (booked) and unspent.
1. The transaction is spending the entirety of the funds of the referenced UTXOs to the outputs.
1. The address type of the referenced UTXO must match the signature type contained in the corresponding _Signature Unlock Block_.
1. The _Signature Unlock Blocks_ are valid, i.e. the signatures prove ownership over the addresses of the referenced UTXOs.

If a transaction passes the semantic validation, its referenced UTXOs *shall* be marked as spent and the corresponding new outputs *shall* be booked/specified in the ledger. 

Transactions that do not pass semantic validation *shall* be discarded. Their UTXOs are not marked as spent and neither are their outputs booked into the ledger. Moreover, their messages *shall* be considered invalid.

# Ledger State

The introduction of a voting-based consensus requires a fast and easy way to determine a node's initial opinion for every received transaction. This includes the ability to both detect double spends and transactions that try to spend non-existing funds. 
These conditions are fulfilled by the introduction of an Unspent Transaction Output (UTXO) model for record-keeping, which enables the validation of transactions in real time.

The concept of UTXO style transactions is directly linked to the creation of a directed acyclic graph (DAG), in which the vertices are transactions and the links between these are determined by the outputs and inputs of transactions. 

To deal with double spends and leverage on certain properties of UTXO, we introduce the Realities Ledger State. 

## Realities Ledger State 

In the Realities Ledger State, we model the different perceptions of the ledger state that exist in the Tangle. In each “reality” on its own there are zero conflicting transactions. 
Each reality thus forms an in itself consistent UTXO sub-DAG, where every transaction references any other transaction correctly.

Since outputs of transactions can only be consumed once, a transaction that double spends outputs creates a persistent branch in a corresponding UTXO DAG. Each branch receives a unique identifier `branchID`. These branches cannot be merged by any vertices (transactions). 
A transaction that attempts to merge incompatible branches fails to pass a validity check and is marked as invalid.

The composition of all realities defines the Realities Ledger State. 

From this composition nodes are able to know which possible outcomes for the Tangle exist, where they split, how they relate to each other, if they can be merged and which messages are valid tips. All of this information can be retrieved in a fast and efficient way without having to walk the Tangle. 

Ultimately, for a set of competing realities, only one reality can survive. It is then up to the consensus protocol to determine which branch is part of the eventually accepted reality.

In total the ledger state thus involves three different layers:
* the UTXO DAG,
* its extension to the corresponding branch DAG,
* the Tangle which maps the parent relations between messages and thus also transactions.

## The UTXO DAG

The UTXO DAG models the relationship between transactions, by tracking which outputs have been spent by what transaction. Since outputs can only be spent once, we use this property to detect double spends. 

Instead of permitting immediately only one transaction into to the ledger state, we allow for different versions of the ledger to coexist temporarily. 
This is enabled by extending the UTXO DAG by the introduction of branches, see the following section. We can then determine which conflicting versions of the ledger state exist in the presence of conflicts.

### Conflict Sets and Detection of Double Spends

We maintain a list of consumers `consumerList` associated with every output, that keeps track of which transactions have spent that particular output. Outputs without consumers are considered to be unspent outputs. Transactions that consume an output that have more than one consumer are considered to be double spends. 

If there is more than one consumer in the consumer list we *shall* create a conflict set list `conflictSet`, which is identical to the consumer list. The `conflictSet` is uniquely identified by the unique identifier `conflictSetID`. Since the `outputID` is directly and uniquely linked to the conflict set, we set `conflictSetID=outputID`.

## Branches

The UTXO model and the concept of solidification, makes all non-conflicting transactions converge to the same ledger state no matter in which order the transactions are received. Messages containing these transactions could always reference each other in the Tangle without limitations.

However, every double spend creates a new possible version of the ledger state that will no longer converge. Whenever a double spend is detected, see the previous section, we track the outputs created by the conflicting transactions and all of the transactions that spend these outputs, by creating a container for them in the ledger which we call a branch. 

More specifically a container `branch` *shall* be created for each transaction that double spends one or several outputs, or if transactions aggregated those branches.
Every transaction that spends directly or indirectly from a transaction in a given `branch`, i.e. is in the future cone in the UTXO DAG of the double-spending transaction that created `branch`, is also contained in this `branch` or one of the child branches.
A branch that was created by a transaction that spends multiple outputs can be part of multiple conflict sets.

Every branch *shall* be identified by the unique identifier `branchID`. We consider two kinds of branches: conflict branches and aggregated branches, which are explained in the following sections.

### Conflict Branches 

A conflict branch is created by a corresponding double spend transaction. Since the transaction identifier is unique, we choose the transaction id `transactionID` of the double spending transaction as the `branchID`.

Outputs inside a branch can be double spent again, recursively forming sub-branches. 

On solidification of a message, we *shall* store the corresponding branch identifier together with every output, as well as the transaction metadata to enable instant lookups of this information. Thus, on solidification, a transaction can be immediately associated with a branch. 


### Aggregated Branches

A transaction that does not create a double spend inherits the branches of the input's branches. In the simplest case, where there is only one input branch the transaction inherits that branch. 

If outputs from multiple non-conflicting branches are spent in the same transaction, then the transaction and its resulting outputs are part of an aggregated branch. This type of branch is not part of any conflict set. Rather it simply combines the perception that the individual conflict branches associated to the transaction's inputs are the ones that will be accepted by the network. Each aggregated branch *shall* have a unique identifier `branchID`, which is the same type as for conflict branches. Furthermore the container for an aggregated branch is also of type `branch`. 

To calculate the unique identifier of a new aggregated branch, we take the identifiers of the branches that were aggregated, sort them lexicographically and hash the concatenated identifiers once

An aggregated branch can't aggregate other aggregated branches. However, it can aggregate the conflict branches that are part of the referenced aggregated branch. 
Thus aggregated branches have no further branches as their children and they remain tips in the branch DAG. Furthermore, the sortation of the `branchID`s in the function `AggregatedBranchID()` ensures that even though messages can attach at different points in the Tangle and aggregate different aggregated branches they are treated as if they are in the same aggregated branch **if** the referenced conflict branches are the same. 

These properties allow for an efficient reduction of a set of branches. In the following we will require the following fields as part of the branch data: 
* `isConflictBranch` is a boolean flat that is `TRUE` if the branch is a conflict branch or `FALSE` if its an aggregated branch.
* `parentBranches` contains the list of parent conflict branches of the branch, i.e. the conflict branches that are directly referenced by this branch.

Then the following function takes a list of branches (which can be either conflict or aggregated branches) and returns a unique set of conflict branches that these branches represent. This is done by replacing duplicates and extracting the parent conflict branches from aggregated branches. 


```vbnet
FUNCTION reducedBranches = ReduceBranches(branches)
    FOR branch IN branches
        IF branch.isConflictBranch
            Append(reducedBranches,branch)
        ELSE
            FOR parentBranch IN branch.parentBranches
                IF NOT (parentBranch IN reducedBranches)
                    Append(reducedBranches,parentBranch)
    
    RETURN reducedBranches
```

### The Branch DAG

A new branch is created for each transaction that is part of a conflict set, or if a transaction aggregates branches.
In the branch DAG, branches constitute the vertices of the DAG. A branch that is created by a transaction that is spending outputs from other branches has edges pointing to those branches.
The branch DAG maps the UTXO DAG to a simpler structure that ignores details about relations between transactions inside the branches and instead retains only details about the interrelations of conflicts.
The set of all non-conflicting transactions form the master branch. Thus, at its root the branch DAG has the master branch, which consists of non-conflicting transaction and resolved transactions. From this root of the branch DAG the various branches emerge. 
In other words the conflict branches and the aggregated branches appear as the children of the master branch. 

### Detecting Conflicting Branches

Branches are conflicting if they, or any of their ancestors, are part of the same conflict set.
The branch DAG can be used to check if branches are conflicting, by applying an operation called normalization, to a set of input branches.
From this information we can identify messages or transactions that are trying to combine branches belonging to conflicting double spends, and thus introduce an invalid perception of the ledger state.

Since branches represent the ledger state associated with a double spend and sub-branches implicitly share the perception of their parents, we define an operation to normalize a list of branches that gets rid of all branches that are referenced by other branches in that list. The function returns `NULL` if the branches are conflicting and can not be merged.

### Merging of Branches Into the Master Branch

A branch gains approval weight when messages from (previously non-attached) `nodeID`s attach to messages in the future cone of that branch. Once the approval weight exceeds a certain threshold we consider the branch as confirmed.
Once a conflict branch is confirmed, it can be merged back into the master branch. Since the approval weight is monotonically increasing for branches from the past to the future, branches are only merged into the master branch.
The loosing branches and all their children branches are booked into the container `rejectedBranch` that has the identifier `rejectedBranchID`.
