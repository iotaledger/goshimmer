# Epochs

Original
https://jamboard.google.com/d/1pb6-1M5HjNiRuozAYQNKE5g1bcj8v6TClJdbTsaTBY0/viewer?f=1

1 of Dec
https://jamboard.google.com/d/1i8k_SZpts7h1zpjuK_QHaMmSj4J4djk0lUF-DKYiE6c/edit?usp=sharing


## Motivation
Messages in the Tangle can reach several degrees of finality (TODO: link Tangle specs) which are determined by a confidence level based on active consensus mana approving any given message. It is, however, not clear how consensus on active consensus mana at any given point in time can be found when nodes' perceptions can differ. Additionally, the solidification process requires consensus mana to determine correct branches of the Tangle while not having solidified anything yet, thus not having any perception on (active) consenus mana. 

We introduce the concept of **Epochs** to solve these problems and potentially enable future use cases such as
- **committee selection:** a committee elected as the x% highest active consensus mana holders can easily be formed. It can already be used to establish a threshold for FPC statements. 
- **finality gadget:** assuming that a committee exists it could publish regular statements about messages and enable non-probabilistic finality.
- **simplified payment verification (SPV):** Finality gadget statements could be used to enable (ad-hoc) nodes to verify the finality of messages without the need to download the entire Tangle. 

## What are epochs?
Epochs are universal time intervals that group messages in the tangle based on their timestamps.
 - An epoch is identified by its unique epoch index. Epoch indices are strictly increasing with respect to time.
 - Every time interval `i` , a new epoch is started and the previous ends.
 - A message `M` belongs to an `Epoch X`, if it is confirmed and its timestamp falls into the time window of 
 `Epoch X` such that T(`M`)∈ [t<sub>x-1</sub>, t<sub>x</sub>), where
   - T(`M`) is the timestamp of message `M`,
   - t<sub>x-1</sub> is the end of the previous epoch,
   - and t<sub>x</sub> is the end of `Epoch X`.
 - The start of the network corresponds to t<sub>1</sub>, that is the end of `Epoch 1`.
 - `Epoch 0` and `Epoch 1` are special epochs, since they only contain the genesis message(s), and their content is defined before starting the network. These two epochs might be used to initialize the network and active consensus mana states to a desired values.
 - For every network, the end of `Epoch 0` should be defined as an arbitrary point in time, similarly to how [Unix epoch time](https://en.wikipedia.org/wiki/Unix_time) is defined as 00:00:00 UTC on 1 January 1970.
 - Epoch interval `i` should also be chosen arbitrary, but taking into account the chosen exponential moving average coefficient for consensus mana.

Figure 1 gives a brief overview of how the Tangle is divided into epochs:

![](https://i.imgur.com/5mZYAO8.png)


`Epoch 0` contains the genesis message(s), that hold the genesis output(s). By allowing multiple outputs to exist before the start of the network, a desired initial state for the network can be set.
`Epoch 2` is the first epoch after the start of the network, when nodes may start issuing messages. Note, that a message can be valid with one strong parent only.

Upon processing a message and verifying its timestamp as described in [Timestamp Specification](https://github.com/iotaledger/goshimmer/pull/742), the message is solidified and can be added to the epoch. 
`Epoch 2` ends at t<sub>2</sub>, but it can happen that a message is issued just before t<sub>2</sub>, therefore it reaches most nodes and gets solidified during `Epoch 3`. In this case, the node can still determine the correct epoch the message belongs to due to the consensus on the message timestamp. This also means, that finalizing an epoch (declaring that no more messages can be added to it) is delayed after the end of the epoch by at least `w` time (`w` is from [Timestamp Specification](https://github.com/iotaledger/goshimmer/pull/742)).

As it will be described in more details later, this delay is the reason for using the epoch before the preveious epoch for active consensus mana calculation in the present.
## Active consensus mana in an epoch
### Definition
Active consensus mana of a `Node A` in `Epoch X` is defined as the consensus mana of the node at t<sub>x</sub>, if and only if there is at least one message in `Epoch X` that was issued by `Node A`.

Therefore, even if `Node A` has consensus mana greater than zero at t<sub>x</sub>, it can be considered dormant (read not active) when it did not issue anything during the epoch.

All nodes that have active consensus mana in the epoch form the set of active consensus mana nodes, or `Active Consensus Mana Set (ACMS)`. It is possible to determine each node's weight in the set by calculating the total active consensus mana of the set.

Determining the `ACMS` in an epoch can already be considered as a committee selection process (`ACMS` nodes will drive the approval weight mechanism), but it is also possible to select only a portion of the set, for example the top `N` active consensus mana holders. 

### Calculation
Active consensus mana calculation for `Epoch X`:

 - collects all nodes that have greater than zero consensus mana at t<sub>x</sub>,
 - and filters out nodes that did not issue at least one message that is in `Epoch X`.

The remaining set of nodes form the `Active Consensus Mana Set (ACMS)` for `Epoch X`.

Calculation can only be carried out when `Epoch X` is finalized, that is, at least `w` time after t<sub>x</sub>.
Furthermore, the calculation has to be able to determine consensus mana of nodes at any time in the past, for example at t<sub>x</sub>.

Let's examine an example from Figure 1:

![](https://i.imgur.com/5mZYAO8.png)

`Epoch 0` and `Epoch 1` are special in the sense, that their `ACMSs` are given in the genesis snapshot, there is no need to try to calculate it.
Let's assume, that the for both `Epoch 0` and `Epoch 1`, the `Green Node` alone forms the `ACMS`, and only this node has consensus mana at t<sub>1</sub>.

Now, let's try to calculate the `ACMS` for `Epoch 2`:
 - Transaction in `Message E` pledges consensus mana to the `Blue Node`.
 - We wait until t<sub>2</sub> + `w` to start the calculation to make sure that no more valid messages belonging to `Epoch 2` will appear in the network.
 - At t<sub>2</sub>, both `Green Node` and `Blue Node` have consensus mana.
 - Both `Green Node` (`B`, `C`) and `Blue Node` (`A`, `D`, `E`) have issued messages that are part of `Epoch 2`.

The `ACMS` nodes for `Epoch 2` therefore are {`Green Node`,`Blue Node`}, with their respective consensus mana values at t<sub>2</sub>.

It worths exploring the calculation for `Epoch 3`:
 - Transaction in `Message K` pledges consensus mana to the `Yellow Node`.
- We wait until t<sub>3</sub> + `w` to start the calculation to make sure that no more valid messages belonging to `Epoch 3` will appear in the network.
- At t<sub>3</sub>, `Green Node`, `Blue Node` and `Yellow Node` have consensus mana.
- `Blue Node` doesn't have any messages in `Epoch 3`, therefore it is excluded from the active consensus mana set calculation.
- `Red Node` has `Message J` in `Epoch 3`, but doesn't have consensus mana at t<sub>3</sub>, therefore it is excluded from the active consensus mana set calculation.

The `ACMS` nodes for `Epoch 3` therefore are {`Green Node`,`Yellow Node`}, with their respective consensus mana values at t<sub>3</sub>.

### Use Cases
#### Approval Weight
Confirmation of messages depends on how much active consensus mana approves them. An approval is expressed as a message issued by a node that is in the currently used `ACMS`, directly or indirectly approving the given message (it is in its future cone).

It is important to note, that the currently used `ACMS` is always the `ACMS` of the last epoch with the same parity as the current. In `Epoch X`, `ACMS(X-2)` is used.

For example, on Figure 1, `Message A` is confirmed in `Epoch 2`, because `Message C` approves it. `Message C` is issued by the `Green Node`, which is the only node in `ACMS(0)`, therefore all active consensus mana approves `Message A`.

#### Committee Selection for FPC Statements
To reduce voting overhead of high consensus mana nodes during FPC rounds, and to help synchronizing nodes decide on conflicts in the past without re-triggering voting, the concept of FPC statements can be introduced.

High consensus mana nodes may publish their opinion about voting (conflict or timestamp) as a special type of message, an `FPC Statement Message` in the Tangle. Nodes consuming these statements should be able to determine which statements to take into account.

A committee formed from the top mana holders in `ACMS` could serve this purpose. A node would decide to only consider FPC statement messages that are issued by the member of the committee.

#### Finality Gadget
Confirmation of messages results in probabilistic consensus because in order for a message to get confirmed, a certain threshold of active consensus mana should approve it. If however, it is required that all active consensus mana approves the given message, consensus becomes determininstic. Once all active consensus mana approves the message, we can be sure, that this messages will not be orphaned, since `ACMS` drives the growing of the Tangle through the confirmation mechanism.

Alternatively, `ACMS` nodes may regularly publish `Finality Statement Messages` in the Tangle that contain message IDs that the `ACMS` nodes consider `LIKED` with a level of knowledge 3. Since `ACMS` drives consensus, if a message ID is present in the `Finality Statement Messages` of all `ACMS` nodes, it can be assumed that the message in final without walking or examining the Tangle.
*Sidenote: this might be very expensive for `ACMS` nodes.*

#### SPV (help me out here Jonas)

### Possible Attack Vectors
#### DDOS Attack on ACMS
Observe, that in order to have confirmed messages in an epoch, the `ACMS` nodes that are used to calculate the approval weight need to be online and issue messages regularly into the future cone of new messages.
Since during an `Epoch X`, the `ACMS` of `Epoch X-2` (`ACMS(X-2)`) is used for calculating the approval weight, and just like an honest node, the attacker also knows which nodes belong to the set, it could organize a DDOS attack against those specific nodes. If successful, `ACMS(X-2)` nodes can't issue messages in the network, and no new message is confirmed  in `Epoch X` since the mana threshold will never be reached. Continuing this logic, the `Epoch X` will contain no messages since nothing gets confirmed, and therefore the `ACMS(X)` will be empty, which blocks future confirmations as well. The network stops operating.

Above scenario can be prevented, if nodes in `ACMS(X-2)`can be brought back online an issue messages until t<sub>X</sub>, or nodes in `ACMS(X-1)` can issue messages in [t<sub>X</sub>, t<sub>X</sub> + `w`].

## Bootstrapping (think more)
### Snapshot
A snapshot removes old messages from the node's database. Those messages may contain transactions, that in turn may contain unspent outputs, which must be preserved. A snapshot therefore should contain all unspent outputs at the time of the snapshot, plus additional information about the `pledgeIDs` and `timestamp` of the transaction that created the outputs, because this information is required when the output is spent and the base mana vector is updated:
 - Who to revoke consensus mana from?
 - How much pending mana the output generates for access mana?

Additionally, if the snapshot happens in `Epoch X`, `ACMS(X-2)` and `ACMS(X-1)` has to be included as by deleting past messages, they could not be recalculated by the node. Also note, that messages that belong to the current epoch can't be deleted. If `ACMS(X-1)` could not calculated yet (because we are in [t<sub>X-1</sub>,t<sub>X-1</sub> +`w` ]), messages from `Epoch X-1` need to be preserved as well.
#### Starting network from scratch
 - When starting the network, the first epoch is always `Epoch 2`.
 - The "Genesis Snapshot" determines `ACMS(0)` and `ACMS(1)`.
 - Modify genesis snaphsot to include several outputs, with information on which nodeIDs get the mana for the output + timestamp of the transaction that created it.
 - Those nodes are considered as active consensus mana nodes in epoch 0 and 1.
 - Active consensus mana nodes should issue fpc statements.
 - In `Epoch 2` and `Epoch 3`, `ACMS(0)` and `ACMS(1)` nodes has to issue messages respectively, otherwise nothing gets confirmed and the Tangle can't grow.

#### Joining the network later (only genesis snapshot)
 - Suppose we are already in `Epoch 10`.
 - A new node joins the network. It has the "genesis snapshot" loaded, as it ships with the node software. It knows `ACMS(0)` and `ACMS(2)`, the genesis message(s) and output(s).
 - The node starts receiving messages from neighbors that are in `Epoch 10`. Since epochs are universal, the node knows that it is missing messages, because the oldest confirmed message in it's database is from before `Epoch 2`.
 - The node starts requesting missing messages based on the parents of the received messages.
 - Eventually, it should get the first solidifiable message, that attach to the genesis messages. It can determine the approval weight of the message because it will be in `Epoch 2`, and it knows `ACMS(0)`.
 - As solidification progresses from the genesis, the node performs the usual way of calculating mana, `ACMS` and approval weight.
 - Eventually, the node will catch up to `Epoch 10` and along the way will have calculated `ACMS(8)` and `ACMS(9)`.

#### Joining the network later (one or more snapshot happened since the genesis snapshot)
 - Snapshot should contain `ACMS` of the previous two epochs before the snapshot, and information on pledge nodeIDs and tx timestamp of the transaction -> based on the outputs you could calculate the consensus mana.
 - Where do you get the snapshot file from? How do you know it can be trusted? Regular snapshots that remove messages up to an epoch? `ACMS` for the previous two epochs plus the ledger state should be the same -> ask it from several nodes and compare? Or just compute the hash of this essential data, ask several nodes to give you their hash of the same and compare?


## Challenges
### Setting epoch interval
### EMA coefficients for consensus mana
### How to calculate consensus mana vector at a given point in time (end of epoch)
We could keep a log of mana pledge and revoke events, and consume the ones corresponding to the given epoch only (timestamp of tx that created the event to the rescue) when calculating the consensus mana.
Save the timstamp of the events to be able to reconstruct effective values.
wait until `MaximumParentsAge` to be sure that we eill not see any more transactions that belongs to the previous epoch.
## Epoch plugin in GoShimmer
TO BE WRITTEN
## Dependencies
### Message timestamps
Message timestamps are currently implemented in GoShimmer (`Issuing Time` field of a transaction), but there is no consensus on timestamp. [Message timestamp proposal](https://github.com/iotaledger/goshimmer/pull/742) aims to provide the specification for implementation.

### Consensus Mana
Consensus mana vector should have a getAllMana() method to get all mana values with the same `LastUpdated` times.

## Questions
 - when there are no previous epochs in the snapshot regarding time, you essentially joined at the beginning, default to 2,1,0...
 - To be considered an active node during the epoch, should your message be confirmed (approved by x mana threshold) or would it be enough as a proof of activity to say that you posted it? => if it is not confirmed, and there was a snapshot, it won't be visible to others joining later.
 - How do PoS systems protect the chosen leader(s)' physical infrastructure? If I know which computer is going to produce the next block, can I just DDOS it?
 - What happens if an `ACMS` node doesn't have access mana to issue messages?
 - What is the maximum time that can pass after an epoch in which messages in the epoch can still confirm? When do we declare that no more messages will be confirmed in an epoch?