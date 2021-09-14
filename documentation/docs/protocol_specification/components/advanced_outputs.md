---
description: IOTA strives to provide output types beyond the basic functionality of a cryptocurrency application such as Smart Contracts.
image: /img/protocol_specification/bob_alias.png
keywords:
- smart contract chain
- state metadata
- state controller
- governance controller
- alias
- smart contract 
- transactions
- NFT
---
# UTXO Output Types

## Motivation

In the previous [section](ledgerstate.md) two basic output types were introduced that enable the use of the UTXO ledger
as a payment application between addresses. Each `SigLockedSingleOutput` and `SigLockedAssetOutput` encodes a list of
balances and an address in the output. The output can be unlocked by providing a valid signature for the address, hence
only the owner of the address can initiate a payment.

While these two output types provide the basic functionality for a cryptocurrency application, IOTA aims to strive
for more. The first and foremost application the UTXO ledger should support besides payments is the IOTA Smart Contract
Protocol (ISCP). Due to the lack of total ordering of the Tangle (that is a direct result of the scalable, parallel
architecture), it is not possible to implement Turing-complete smart contracts directly on layer 1. Therefore,
IOTA aims to develop a layer 2 protocol called ISCP for smart contracts.

After carefully evaluating the proposed architecture of ISCP and the required properties of the layer 2 protocol, we
came up with special types of outputs for layer 1 UTXO support: `AliasOutput` and `ExtendedLockedOutput`.
These output types are experimental: the IOTA 2.0 DevNet serves as their testing ground. Bear in mind that there is no
guarantee that they will not change as the protocol evolves.

It will be demonstrated later that these outputs can also be used for enhanced cryptocurrency payment application, such
as conditional payments or time locked sending, but also open up the world of native non-fungible tokens (NFTs).

## Functional Requirements of ISCP

Designing the output types starts with a proper requirement analysis. Below you can read the summary of the functional
requirements imposed by the layer 2 smart contract protocol. You can read more about ISCP
[here](https://blog.iota.org/an-introduction-to-iota-smart-contracts-16ea6f247936/),
[here](https://blog.iota.org/iota-smart-contracts-protocol-alpha-release/)
or check out this [presentation](https://youtu.be/T1CJFr6gz8I).

- Smart contract chains need a globally unique account in the UTXO ledger, that does not change if the controlling entities changes.
- An account state is identified by balances and state metadata.
- Two levels of control: **state controller** and **governance controller**.
- State controller can change state metadata (state transition) and balance (min required).
- Governance controller can change state controller and governance controller.
- An account shall have only one valid state in the ledger.
- Smart contract chain state transitions are triggered by requests in the ledger.
- A request is a ledger entity belonging to the account with tokens and data.
- The account can identify and control requests.
- Fallback mechanism needs to be in place in case the requests are not picked up.
- When request is completed in a state transition, it should be atomically removed from the ledger.

## Output Design

### Introducing Alias Account

Previously, the account concept in the ledger was realized with cryptographic entities called addresses, that are backed
by public and private key pairs. Addresses are present in the ledger through outputs and define who can spend this
output by providing a digital signature.

Addresses are not able to provide the necessary functionality needed for smart contract chain accounts, because:
- addresses change with the rotation of the controlling body (committee),
- and there is no notion of separate control levels for an address account.

We define a new account type in the ledger, called **Alias**, to represent smart contract chain accounts. An alias
account can hold token balances, but also has state metadata, which stores the state of the smart contract chain. 
The alias account defines two to controlling entities: a state controller and a governance controller. The state 
controller can transition the account into a new state, and can manipulate account balances. The governance controller
can change the state controller or the governance controller.

An alias is not a cryptographic entity, but it is controlled via either regular addresses or other aliases.

### Representing a Smart Contract Chain Account in Ledger

An alias is translated into the ledger as a distinct output type, called **AliasOutput**. The output contains:
- the unique identifier of the alias, called **AliasID**,
- the **State Controller** entity,
- **State Metadata**,
- the **Governance Controller**,
- **Governance Metadata**,
- **Immutable Metadata**,
- and token **balances**.

The state controller and governance controller entities can either be private key backed addresses (cryptographic 
entities) or `AliasAddress`, that is the unique identifier of another alias. Note, that an alias cannot be controlled by
its own `aliasID`.

An alias output itself can be regarded as a non-fungible token with a unique identifier `aliasID`, metadata and token
balances. An NFT that can hold tokens, can control its metadata and has a governance model.

Alias output can be created in a transaction that spends the minimum required amount of tokens into a freshly created
alias output. The new transaction output specifies the state and governance controller next to the balances, but aliasID
is assigned by the protocol once the transaction is processed. Once the output is booked, aliasID becomes the hash of
the outputID that created it.

An alias output can only be destroyed by the governance controller by simply consuming it as an input but not creating
a corresponding output in the transaction.

The alias account is transitioned into a new state by spending its alias output in a transaction and creating an
updated alias output with the same aliasID. Depending on what unlocking conditions are met, there are certain
restrictions on how the newly created alias output can look like.

### Consuming an Alias Output

As mentioned above, an alias output can be unlocked by both the state controller and the governance controller.

#### Unlocking via State Controller

When the state controller is an address, the alias output is unlocked by providing a signature of the state controller
address in the output that signs the essence of the transaction. When state controller is another alias, unlocking is
done by providing a reference to the state controller unlocked other alias within the transaction.

When an alias output is unlocked as input in a transaction by the state controller, the transaction must contain a
corresponding alias output. Only the state metadata and the token balances of the alias output are allowed to change,
and token balances must be at least a protocol defined constant.

#### Unlocking via governance controller

The governance controller is either an address, or another alias. In the former case, unlocking is done via the regular
signature. In the latter case, unlocking is done by providing a reference to the unlocked governance alias within the
transaction.

When an alias output is unlocked as input by the governance controller, the transaction doesn't need to have a
corresponding output. If there is no such output in the transaction, the alias is destroyed. If however the output
is present, only the state and governance controller fields are allowed to be changed.

A governance controller therefore can:
- destroy the alias all together,
- assign the state controller of the alias,
- assign the governance controller of the alias.

## Locking Funds Into Aliases

Address accounts in the ledger can receive funds by the means of signature locking. Outputs specify an address field,
which essentially gives the control of the funds of the output to the owner of the address account, the holder of the
corresponding private key.

In order to make alias accounts (smart contract chains) able to receive funds, we need to define a new fund locking
mechanism, called alias locking. An alias locked output can be unlocked by unlocking the given alias output for
state transition in the very same transaction.

An alias account (smart contract chain) can receive funds now, but there are additional requirements to be satisfied 
for smart contracts:
- Alias locked outputs represent smart contract requests, and hence, need to contain metadata that is interpreted on
  layer 2.
- A dormant smart contract chain might never consume alias locked outputs, therefore, there needs to be a fallback
  mechanism for the user to reclaim the funds locked into the request.
- Requests might be scheduled by the user by specifying a time locking condition on the output. The output can not be
  spent before the time locking period expires.

As we can see, there are couple new concepts regarding outputs that we need to support for the smart contract use case:
- **alias locking**
- **metadata tied to output**
- **fallback unlocking mechanism**
- **time locking**

In the next section, we are going to design an **Extended Output** model that can support these concepts.

## Extended Output

An extended output is an output that supports alias locking, output metadata, fallback unlocking mechanisms and time
locking. The structure of an extended output is as follows:

Extended Output:
- **AliasID**: the alias account that is allowed to unlock this output.
- **Token Balances**: tokens locked by the output.
- **Metadata**: optional, bounded size binary data.
- **FallbackAccount**: an alias or address that can unlock the output after **FallbackDeadline**.
- **FallbackDeadline**: a point in time after which the output might be unlocked by **FallbackAccount**.
- **Timelock** (Optional): a point in time. When present, the output can not be unlocked before.

### Unlocking via AliasID

The extended output can be unlocked by unlocking the alias output with aliasID by the state controller within the same
transaction. The unlock block of an extended output then references the unlock block of the corresponding alias output.

Aliases abstract away the underlying address of a smart contract committee, so when a committee is rotated, `aliasID`
stays the same, but the address where the alias points to can be changed.

It is trivial then to define the unique account of a smart contract on layer 1 as the `aliasID`, however, a new locking
mechanism is needed on the UTXO layer to be able to tie funds to an alias.

Previously, only addresses defined accounts in the protocol. Funds can be locked into addresses, and a signature of the
respective address has to be provided in the transaction to spend funds the account.

With the help of aliases, it is possible to extend the capabilities of the protocol to support locking funds into
aliases. This is what we call alias locking. An alias locked output specifies an `aliasID` that can spend the funds
from this output. The owner of the alias account can spend aforementioned alias locked outputs by unlocking/moving the
alias in the very same transaction. We will use the term `ExtendedLockedOutput` for outputs that support alias locking.

Let's illustrate this through a simple example. Alice wants to send 10 Mi to Bob's alias account. Bob then wants to
spend the 10 Mi from his alias account to his address account.

1. Bob creates an alias where `aliasID=BobAliasID` with Transaction A.

[![Bob creates an alias](/img/protocol_specification/bob_alias.png "Bob creates an alias")](/img/protocol_specification/bob_alias.png)

2. Bob shares `BobAliasID` with Alice.
3. Alice sends 10 Mi to Bob by sending Transaction B that creates an `ExtendedLockedOutput`, specifying the balance,
   and `aliasID=BobAliasID`.

[![Alice sends 10 Mi to Bob](/img/protocol_specification/alice_sends_10_mi.png "Alice sends 10 Mi to Bob")](/img/protocol_specification/alice_sends_10_mi.png)

4. Bob can spend the outputs created by Alice by creating Transaction C that moves his `BobAlias` (to the very same
   address), and including the  `ExtendedLockedOutput` with `aliasID=BobAliasID`.

[![Bob can spend the outputs created by Alice by creating Transaction C](/img/protocol_specification/bob_can_spend_outputs_created_by_alice.png "Bob can spend the outputs created by Alice by creating Transaction C")](/img/protocol_specification/bob_can_spend_outputs_created_by_alice.png )

In a simple scenario, a user wishing to send a request to a smart contract creates an extended output. The output
contains the AliasID of the smart contract chain account, the layer 2 request as metadata, and some tokens to pay
for the request. Once the transaction is confirmed, the smart contract chain account "receives" the output. It
interprets the request metadata, carries out the requested operation in its chain, and submits a transaction that
contains the updated smart contract chain state (alias output), and also spends the extended output to increase
the balance of its alias output.

What happens when the smart contract chain goes offline or dies completely? How do we prevent the extended output to
be lost forever?

### Unlocking via Fallback

Extended outputs can also define a fallback account and a fallback deadline. After the fallback deadline, only the
fallback account is authorized to unlock the extended output. Fallback deadline cannot be smaller than a protocol
wide constant to give enough time to the smart contract chain to pick up the request.

Fallback unlocking can either be done via signature unlocking or alias unlocking, depending on the type  of account
specified.

### Timelock

Timelocking outputs is a desired operation not only for smart contracts, but for other use cases as well. A user might
for example scheduled a request to a smart contract chain at a later point in time by timelocking the extended output
for a certain period.

Timelocks can be implemented quite easily if transactions have enforced timestamps: the output can not be unlocked if
the transaction timestamp is before the timelock specified in the output.

## Notes

One of the most important change that the new output types imply is that checking the validity of an unlock block of a
certain consumed input has to be done in the context of the transaction. Previously, an unlock block was valid if the
provided signature was valid. Now, even if the signature is valid for an alias output unlocked for state transition,
additional constraints also have to be met.

## How Does It Work for ISCP?

- The new output types are completely orthogonal to colored coins, ISCP will not rely on them anymore.
- The Alias output functions as a chain constraint to allow building a non-forkable chain of transactions in the
  ledger by the state controller. The alias output holds tokens, that are the balance of the smart contract chain.
  The hash of the smart contract chain state is stored in the alias output, registering each state transition as a
  transaction on the ledger.
- The governance controller of an alias output can change the state controller, meaning that a committee rotation can
  be carried out without changing the smart contract chain account, aliasID.
    - A smart contract chain can be self governed, if the state and governance controllers coincide.
    - A smart contract chain can be governed by an address account, or by another smart contract chain through an 
      alias account.
- Each Extended Output is a request which is “sent” to the alias account. The ISCP can retrieve the backlog of
  requests by retrieving all outputs for the aliasID. Consuming the Extended Output means it is atomically removed
  from the backlog. It can only be done by the state controller, i.e. the committee of the smart contract chain.
- Fallback parameters prevent from losing funds if the committee is inactive for some timeout. After timeout the 
  Extended Output can be unlocked by FallbackAccount, an address or another alias.

## Additional Use Cases

### Delegated Keys

An alias output is controlled by two parties: the state controller and the governance controller. The state controller
can only change the state metadata and the tokens when spending the output, therefore it only has the right to move the
alias to the very same account in a transaction. The governance controller however can change the state controller, or
destroy the alias and hence release the funds locked into it.

This makes it an ideal candidate for mana delegation, that is a crucial part of a mana marketplace. In Coordidice,
moving funds generate access and consensus mana. Alias outputs make it possible to delegate the right to move funds
without losing control over them.

1. An account owning funds create an alias output and locks funds into it. The governance controller of the alias output
   shall be `ownAccount`.
2. An entity in need of mana generated by the locked funds can purchase the right from the governance controller to
   move the alias output, generating mana.
3. Once purchased, the governance controller updates the alias output by specifying the state controller to be
   `buyerAccount`.
4. `buyerAccount` now can move the alias output, but only to its own account. Each move generates (access) mana.
5. Since `ownAccount` is the governance controller, it can revoke `buyerAccount`'s state controlling right at any point
   in time.
6. `ownAccount` can also destroy the alias and "free" the locked funds.

Notes:
- The state controller can redeem funds from the alias output up to the point where only `minimum allowed amount` is
  present in the alias output. Therefore, without additional mechanism, it would only make sense to lock
  `minimum allowed amount` into an alias by the governance controller. This is obviously a drawback, users should not
  be restricted in how many funds they would like to delegate.
- A governance controller can destroy the alias output at any time, which is not desired from the buyer perspective.
  The buyer should be able to buy the right to move the funds for a pre-defined amount of time.

To solve above problems, the `AliasOutput` currently implemented in GoShimmer supports the delegation use case by
introducing two new fields in the output:
- `isDelegated` and
- `delegationTimelock`.

When an alias is delegated, the state controller cannot modify token balances, and the governor can destroy the
output with any balance. However, when delegation time lock is present, the governor is not allowed to unlock the
output until the delegation time expires.

### Non-Fungible Tokens

NFTs are unique tokens that have metadata attached to them. Since an AliasOutput implements a chain constraint in the
UTXO ledger, it is perfectly suited to represent NFTs. The unique identifier of the NFT is the `aliasID` or `AliasAddress`.
The `Immutable Data` field of the output can only be defined upon creation and can't be changed afterwards, therefore
it is perfect to store metadata belonging to the NFT.

The ID of an IOTA NFT is also a valid address, therefore the NFT itself can receive and manage funds and other NFTs as
well. Refer to the [cli-wallet tutorial](../../tutorials/wallet_library.md) for an overview of what you can do with an NFT.

Interestingly, minting an IOTA NFT costs you only the minimum required deposit balance (0.0001 MI at the moment), which
you can take back when you destroy the NFT. This is required so that NFTs are not minted out of thin air, and there are
some IOTAs backing the output. Otherwise, the ledger database could be easily spammed.
Transferring NFTs is also feeless, just like any other transaction in IOTA.

## GoShimmer Implementation

If you are interested, you can find the GoShimmer implementation of the new ouput types in
[output.go](https://github.com/iotaledger/goshimmer/blob/master/packages/ledgerstate/output.go):
 - [AliasOutput](https://github.com/iotaledger/goshimmer/blob/master/packages/ledgerstate/output.go#L947) and
 - [ExtendedLockedOutput](https://github.com/iotaledger/goshimmer/blob/master/packages/ledgerstate/output.go#L1840)