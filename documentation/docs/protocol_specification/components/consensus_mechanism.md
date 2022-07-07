---
description: The consensus mechanism is necessary to achieve agreement among the nodes of the network.  Since the Tangle is only partially ordered we have designed an open and leaderless consensus mechanism which combines FPC and Approval Weight.

keywords:
- node
- approval weight
- conflict
- opinion
- block
- high probability
- active consensus mana
---
# Consensus Mechanism

The consensus mechanism is necessary to achieve agreement among the nodes of the network. In case of a double spend, one way to decide which transaction should be considered valid would be to order them and pick the oldest one. However, the Tangle is only partially ordered. To tackle this problem in the context of the Tangle, we have designed an open and leaderless consensus mechanism that utilizes the Tangle as a medium to exchange votes. Any node can add a block to the Tangle, and each block added to the Tangle represents a virtual vote (i.e. there is no additional overhead to communicate votes) to its entire past.

The consensus mechanism can broadly be devided into consensus on two separate entities. On the one hand, we need to resolve any conflicts on the underlying UTXO ledger to prevent double spends. On the other hand, we need to make sure that blocks within the Tangle are not orphaned. Both are simply derived by observing the Tangle and objectively keeping track of [Approval Weight (AW)](#Approval-Weight-AW) with cMana (more specifically [active cMana](#Active-cMana)) as a Sibyl protection. Once a [conflict](ledgerstate.md#conflicts) (or block) reaches a certain AW threshold, an application can consider it as *confirmed*. To simplify this notion we introduce [Grades of Finality (GoF)](#Grades-of-Finality-GoF) where a higher GoF represents a higher confidence.

| Name                 | Component    | Initial/local opinion | Consensus    | Comparable blockchain mechanism for voting/finality |
| -------------------- | --- | --------------------- | ------------ | --------------------------------------------------- |
| voting on conflicts  | UTXO ledger    | OTV/FPCS              | conflict/tx AW | longest chain rule                                  |
| finality of blocks | Tangle    | inclusion score via tip selection       | block AW   | x block rule                                        |


On an abstract level, a node can be seen as a replicated state machine, just following whatever it receives through the Tangle, and, in case of blocks containing transactions, modifying the UTXO ledger. Only when a node wants to issue a block (read as: *cast a vote*) it needs to evaluate its own local opinion via [modular conflict selection function](#Modular-Conflict-Selection-Function). This decoupling of coming to consensus and setting the initial opinion allows for great flexibility and separation of concerns.





## Approval Weight (AW)
Approval weight represents the [weight](#active-consensus-mana) of conflicts (and blocks), similar to the longest chain rule in Nakamoto consensus. However, instead of selecting a leader based on a puzzle (PoW) or stake (PoS), it allows every node to express its opinion by simply issuing any block and attaching it in a part of the Tangle it *likes* (based on its initial opinion on blocks and possibly utilizing the [like switch](#Like-Switch) to express its opinion on conflicts).

It is important to note that tracking of AW for conflicts and markers/blocks is orthogonal. Thus, a block can reach a high AW whereas its contained payload, e.g., a transaction being a double spend, does not reach any AW on conflict/UTXO level.

### Detailed Design
Approval weight AW increases because of voters (nodes) that cast votes for conflicts and blocks by means of making statements. This is necessary due to the changing nature of cMana over time, which prevents simply counting the AW per conflict or block. Additionally, whenever a node changes its opinion on a conflict, the previous vote needs to be invalidated.

#### Definitions
* **Statement**: A statement is any block issued by a *node*, expressing its opinion and casting a (virtual) vote. It can be objectively ordered by its timestamp, and, if equal, its block ID.
* **Conflict voter**: A conflict voter is a *node* that issued a statement attaching to a conflict, and, thus, voting for it.
* **Marker/block voter**: A marker/block's voter is a *node* that issued a statement directly or indirectly referencing a marker/block, including its issuer.

#### Conflicts
Tracking voters of [conflicts](ledgerstate.md#conflicts) is an effective way of objective virtual voting. It allows nodes to express their opinion simply by attaching a statement to a conflict they like (see [like switch](#Like-Switch)). This statement needs to propagate down the conflict DAG, adding support to each of the conflict's parents. In case a voter changes their opinion, support needs to be revoked from all conflicting conflicts and their children. Thus, a node can only support one conflict of a conflict set.

To make this more clear consider the following example:

[![Conflict Voter](/img/protocol_specification/conflicts.png)](/img/protocol_specification/conflicts.png)



The green node issued **statement 1** and attached it to the aggregated conflict `Conflict 1.1 + Conflict 4.1.1`. Thus, the green node is a voter of all the aggregated conflict's parent conflicts, which are (from top to bottom) `Conflict 4.1.1`, `Conflict 1.1`, `Conflict 4.1`, `Conflict 1`, and `Conflict 4`.

Then, the green node issued **statement 2** and attached it to `Conflict 4.1.2`. This makes the green node a voter of `Conflict 4.1.2`, however, `Conflict 4.1.1` is its conflict conflict and thus support for `Conflict 4.1.1` has to be revoked.

`Conflict 4.1`, `Conflict 4` are parent conflicts of `Conflict 4.1.2`, which the green node is still supporting. Since `Conflict 1.1`, `Conflict 1` are not conflicting to either of `Conflict 4.1.2`'s parents, the green node remains their voter.

Finally, the green nodes issued **statement 3**, which is in `Conflict 2`. Now the green node is a voter of `Conflict 2`, and no longer a voter of `Conflict 1`, since `Conflict 1` is conflicting to `Conflict 2`. Note that, this voter removal will propagate to child conflicts. Thus, the green node is removed from `Conflict 1.1` as well.
`Conflict 3`, `4` and both of their child conflicts have nothing to do with this attachement, the voter status remains.

It is important to notice that the arrival order of the statements does not make a difference on the final outcome. Due to the fact that statements can be ordered objectively, every node in the network eventually comes to the same conclusion as to who is supporting which conflict, even when nodes change their opinions.


##### Calculation of Approval Weight
The approval weight itself is calculated every time a new voter is added/removed to a conflict. The AW for a conflict *B* is calculated as follows:

```
AW(B) = 'active cMana of voters(B)' / 'total active cMana'
```

#### Markers
It would be computationally expensive to track the AW for each block individually. Instead, we approximate the AW with the help of [markers](markers.md). Once a marker fulfills a GoF, the corresponding GoF value is propagated into its past cone until all  blocks have an equal or higher GoF.

Recall that markers are not part of the core protocol. As such, this description is merely an optimization from an implementation standpoint.

Rather than keeping a list of voters for each marker and collecting voters for each marker (which would also be expensive), we keep a list of voters along with its approved marker index for each marker sequence. This approach provides a simple and fast look-up for marker voters making use of the Tangle structure as mapped by the markers.

For each marker sequence, we keep a map of voter to marker index, meaning a voter supports a marker index `i`. This implies that the voter supports all markers with index `<= i`.

Take the figure below as an example: 
![MarkersApprovalWeight SequenceVoters](/img/protocol_specification/MarkersApprovalWeight.png)

The purple circles represent markers of the same sequence, the numbers are marker indices.

Four nodes (A to D) issue statements with past markers of the purple sequence. Node A and D issue blocks having past marker with index 6, thus node A and D are the voters of marker 6 and all markers before, which is 1 to 5. On the other hand, node B issues a block having past marker with index 3, which implies node B is a voter for marker 1 and 2 as well.

This is a fast look-up and avoids walking through a marker's future cone when it comes to retrieving voters for approval weight calculation.

For example, to find all voter of marker 2, we iterate through the map and filter out those support marker with `index >= 2`. In this case, all nodes are its voters. As for marker 5, it has voters node A and D, which fulfill the check: `index >= 5`.

Here is another more complicated example with parent sequences:
![MarkersApprovalWeight SequenceVoters](/img/protocol_specification/MarkersApprovalWeightSequenceVoters.png)

The voter will be propagated to the parent sequence.

Node A issues block A2 having past markers `[1,4], [3,4]`, which implies node A is a voter for marker `[1,1]` to `[1,4]`, `[2,1]` to `[2,3]`, and `[3,4]`  as well as the block with marker `[3,5]` itself.

##### Calculation of Approval Weight
The approval weight itself is calculated every time a new voter is added to a marker. The AW for a marker *M* is calculated as follows:

```
AW(M) = 'active cMana of voters(M)' / 'total active cMana'
```


### Grades of Finality (GoF)
The tracking of AW itself is objective as long as the Tangle converges on all nodes. However, delays, network splits and ongoing attacks might lead to differences in perception so that a finality can only be expressed probabilistically. The higher the AW, the less likely a decision is going to be reversed. To abstract and simplify this concept we introduce the GoF. Currently, they are simply a translation of AW thresholds to a GoF, but one can imagine other factors as well.

**Block / non-conflicting transaction**
GoF | AW
-- | --
0 | < 0.25
1 | >= 0.25
2 | >= 0.45
3 | >= 0.67

**Conflict / conflicting transaction**
GoF | AW
-- | --
0 | < 0.25
1 | >= 0.25
2 | >= 0.45
3 | >= 0.67

These thresholds play a curcial role in the safety vs. liveness of the protocol, together with the exact workings of [active cMana](#Active-cMana). We are currently investigating them with in-depth simulations.
* The higher the AW threshold the more voters a conflict or block will need to reach a certain GoF -> more secure but higher confirmation time.
* As a consequence of the above point, TangleTime will be tougher to advance; making the cMana window more likely to get stuck and confirmations to halt forever.

An application needs to decide when to consider a block and (conflicting) transaction as *confirmed* based on its safety requirements. Conversely, a block or conflict that does not gain enough AW stays pending forever (and is orphaned/removed on snapshotting time).


## Modular Conflict Selection Function
The modular conflict selection function is an abstraction on how a node sets an initial opinion on conflicts. By decoupling the objective perception of AW and a node's initial opinion, we gain flexibility and it becomes effortless to change the way we set initial opinions without modifying anything related to the AW.


### Pure On Tangle Voting (OTV)
The idea of pure OTV is simple: set the initial opinion based on the currently heavier conflict as perceived by AW. However, building a coherent overall opinion means that conflicting realities (possible outcomes of overlapping conflict sets) can not be liked at the same time, which makes finding the heaviest conflict to like not as trivial as it may seem.

In the examples below, a snapshot at a certain time of a UTXO-DAG with its conflicts is shown. The gray boxes labelled with `O:X` represent an output and and arrow from an output to a transaction means that the transaction is consuming this output. An arrow from a transaction to an output creates this output. An output being consumed multiple times is a conflict and the transactions create a conflict, respectively. The number assiged to a conflict, e.g., `Conflict A = 0.2`, defines the currently perceived Approval Weight of the conflict. A conflict highlighted in **bold** is the outcome of applying the pure OTV rules, i.e., the conflicts that are liked from the perspective of the node.

**Example 1**
The example below shows how applying the heavier conflict rule recursively results in the end result of `A`, `C`, `E`, and thus the aggregated conflict `C+E` being liked. Looking at the individual conflict weights this result might be surprising: conflict `B` has a weight of `0.3` which is bigger than its conflict conflict `A = 0.2`. However, `B` is also in conflict with `C` which has an even higher weight `0.4`. Thus, `C` is liked, `B` cannot be liked, and `A` suddenly can become liked again.

`E = 0.35` is heavier than `D = 0.15` and is therefore liked. An (aggregated) conflict can only be liked if all its parents are liked which is the case with `C+E`.

![OTV example 1](/img/protocol_specification/otv-example-1.png)

**Example 2**
This example is exactly the same as example 1, except that conflict `C` has a weight of `0.25` instead of `0.4`. Now the end result is conflicts `B` and `E` liked. Conflict `B` is heavier than conflict `C` and `A` (winning in all its conflict sets) and becomes liked. Therefore, neither `A` nor `C` can be liked.


Again, `E = 0.35` is heavier than `D = 0.15` and is therefore liked. An (aggregated) conflict can only be liked if all its parents are liked which is not the case with `C+E` in this example.

![OTV example 2](/img/protocol_specification/otv-example-2.png)



### Metastability: OTV and FPCS
Pure OTV is susceptible to metastability attacks: If a powerful attacker can keep any conflict of a conflict set reaching a high enough approval weight, the attacker can prevent the network from tipping to a side and thus theoretically halt a decision on the given conflicts indefinitely. Only the decision on the targeted conflicts is affected but the rest of the consensus can continue working. By forcing a conflict to stay unresolved, an attacker can, at most, prevent a node from pruning resources related to the pending decision.

In order to prevent such attacks from happening we are planning to implement FPCS with OTV as a conflict selection function. A more detailed description can be found [here](https://iota.cafe/t/on-tangle-voting-with-fpcs/1218).




## Like Switch
Without the like switch, blocks vote for conflicts simply by attaching in their underlying conflict's future cone. While this principle is simple, it has one major flaw: the part of the Tangle of the losing conflict is abandoned so that only the *valid* part remains. This might lead to mass orphanage of "unlucky" blocks that happened to first vote for the losing conflict. With the help of weak parents these blocks might be *rescued* without a reattachment but the nature of weak parents makes it necessary so that every block needs to be picked up individually. Next to the fact that keeping such a weak tip pool is computationally expensive, it also open up orphanage attack scenarios by keeping conflicts undecided (metastability attack turns into orphanage attack).

The like switch is a special type of parent reference that enables  keeping everything in the Tangle, even conflicting transactions that are not included into the valid ledger state by means of consensus. Therefore, it prevents mass orphanage and enables a decoupling of **voting on conflicts (UTXO ledger)** and **finality of blocks / voting on blocks (Tangle)**. It makes the overall protocol (and its implementation) not only more efficient but also easier to reason about and allows for lazy evaluation of a node's opinion, namely only when a node wants to issue a block (read as: *cast a vote*).


From a high-level perspective, the like switch can be seen as a set of rules that influence the way a block inherits its conflicts. Using only strong parents, a block inherits all its parents' conflicts. A like parent retains all the properties of the strong parent (i.e., inherit the conflict of said parent) but additionally it means to exclude all conflicts that are conflicting with the liked conflict.
Through this mechanism, it becomes possible to attach a block anywhere in the Tangle but still only vote for the conflicts that are liked. Thus, decoupling of block AW and conflict AW.

**Examples**
To make this more clear, let's consider the following examples. The conflicts `A` and `B`, as well as `C` and `D` form an independent conflict set, respectively. The `Conflict Weights` are the weights as perceived by the node that currently wants to create a block.


**Block creation**
A node performs random tip selection (e.g. URTS) and in this example selects blocks `5` and `11` as strong parents. Now the node needs to determine whether it currently *likes* all the conflicts of the selected blocks (`red, yellow, green`) in order to apply the like switch (if necessary) and vote for its liked conflicts.

![Like switch: block creation undecided](/img/protocol_specification/like-switch-block-creation-1.png)

When performing the conflict selection function with pure OTV it will yield the result:
- `red` is disliked, instead like `purple`
- `green` is disliked, instead like `yellow`

Therefore, the block needs to set two like references to the blocks that introduced the conflict (first attachment of the transaction). The final result is the following:
- conflicts `purple` and `yellow` are supported
- block `5` (and its entire past cone, `3`, `2`, `1`) is supported
- block `11` (and its entire past cone, `6`, `4`, `2`, `1`) is supported
- block `6` (and its entire past cone, `4`, `2`, `1`)
- block `7` (and its entire past cone, `1`)

![Like switch: block creation](/img/protocol_specification/like-switch-block-creation-2.png)


**Block booking**
On the flip side of block creation (casting a vote) is applying a vote when booking a block, where the process is essentiall just reversed. Assuming a node receives block `17`. First, it determines the conflicts of the strong parents (`red`, `yellow`, `purple`) and like parents (`red`). Now, it removes all conflicts that are conflicting with the like parents' conflicts (i.e., `purple`) and is left with the conflicts `red` and `yellow` (and adds the like conflicts to it, which in this case is without effect). If the resulting conflict is `invalid`, (e.g., because it combines conflicting conflicts) then the block itself is considered invalid because the issuer did not follow the protocol.

In this example the final result is the following (block `17` supports):
- conflicts `red` and `yellow`
- block `16` and its entire past cone
- block `11` and its entire past cone
- block `4` and its entire past cone

![Like switch: block booking](/img/protocol_specification/like-switch-block-booking.png)

## Active cMana
The consensus mechanism weighs votes on conflicts (blocks in future cone with like switch) or block inclusion (all blocks in future cone) by the limited resource cMana, which is thus our Sybil-protection mechanism. cMana can be pledged to any nodeID, including offline or non-existing nodes, when transferring funds (in the proportion of the funds) and is instantly available (current implementation without EMA). Funds might get lost/locked over time, and, therefore, the total accessible cMana declines as well. **Consequently, a fixed cMana threshold cannot be used to determine a voting outcome.**

Finalization of voting outcomes should happen once a conflict is *sufficiently* decided and/or a block is *sufficiently* deep in the Tangle. However, measuring AW in terms of cMana alone does not yield enough information. Therefore, a degree/grade of finalization and of voting outcome in relation to *something*, e.g., **recently active nodes**, is preferred. This measure should have the following properties:
- security/resilience against various attacks
- lose influence (shortly) after losing access to funds
- no possibility of long-range attacks
- no too quick fluctuations
- real incentives


### Current Implementation
Active cMana in GoShimmer basically combines two components in an active cMana WeightProvider: the TangleTime and the current state of cMana. A node is considered to be active if it has issued any block in the last `activeTimeThreshold=30min` with respect to the TangleTime. The total active consensus mana is, therefore, the sum of all the consensus mana of each active node.

#### TangleTime
The TangleTime is the issuing time of the last confirmed block. It cannot be attacked without controlling enough mana to accept incorrect timestamps, making it a reliable, attack-resistant quantity.

![Tangle Time](/img/protocol_specification/tangle_time.jpg)

#### cMana
The current state of cMana is simply the current cMana vector, at the time the active cMana is requested.


#### Putting it together
Every node keeps track of a list of active nodes locally. Whenever a node issues a block it is added to the list of active nodes (nodeID -> issuing time of latest block). When the active cMana is requested all relevant node weights are returned. Relevant here means the following:
- the node has more than `minimumManaThreshold=0` cMana to prevent bloating attacks with too little cMana
- there is a block that fulfills the condition `issuing time <= TangleTime && TangleTime - issuing time <= activeTimeThreshold` where `activeTimeThreshold=30min` (see the following example, blocks `1` and `3` are not within the window)

![Active cMana window](/img/protocol_specification/active-cMana-window.png)


### Example
When syncing (`TT=t0`) and booking a block from time `t1`, active cMana is considered from `t0-activeTimeThreshold`. Once this block gets confirmed, the TangleTime advances to `TT=t1`. For the next block at `t2`, `TT=t1-activeTimeThreshold` will be considered. Using active cMana in this way, we basically get a sliding window of how the Tangle emerged and *replay* it from the past to the present.


### Pros
- replaying the Tangle as it emerged
- always use cMana from the current perspective
- relatively simple concept

### Cons
- active cMana does not yield sufficient information (e.g. when eclipsed), it might look like something is 100% confirmed even though only 2% of the total cMana are considered active.
- active cMana might change quickly nodes with high mana suddenly become active
- if nodes are only able to issue blocks when "in sync" and no block gets confirmed within that time, nobody might be able to issue blocks anymore
- if a majority/all active cMana nodes go offline _within the active cMana window_, consensus will halt forever because the TangleTime can never advance unless a majority of these nodes move the TangleTime forward

This reflects the current implementation and we are currently investigating active cMana with in-depth simulations to improve the mechanism.