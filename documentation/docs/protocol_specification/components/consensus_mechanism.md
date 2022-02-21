---
description: The consensus mechanism is necessary to achieve agreement among the nodes of the network.  Since the Tangle is only partially ordered we have designed an open and leaderless consensus mechanism which combines FPC and Approval Weight.

keywords:
- node
- approval weight
- branch
- opinion
- message
- high probability
- active consensus mana
---
# Consensus Mechanism

The consensus mechanism is necessary to achieve agreement among the nodes of the network. In case of a double spend, one way to decide which transaction should be considered valid would be to order them and pick the oldest one. However, the Tangle is only partially ordered. To tackle this problem in the context of the Tangle, we have designed an open and leaderless consensus mechanism that utilizes the Tangle as a medium to exchange votes. Any node can add a message to the Tangle, and each message added to the Tangle represents a virtual vote (i.e. there is no additional overhead to communicate votes) to its entire past.

The consensus mechanism can broadly be devided into consensus on two separate entities. On the one hand, we need to resolve any conflicts on the underlying UTXO ledger to prevent double spends. On the other hand, we need to make sure that messages within the Tangle are not orphaned. Both are simply derived by observing the Tangle and objectively keeping track of [Approval Weight (AW)](#Approval-Weight-AW) with cMana (more specifically [active cMana](#Active-cMana)) as a Sibyl protection. Once a [branch](ledgerstate.md#branches) (or message) reaches a certain AW threshold, an application can consider it as *confirmed*. To simplify this notion we introduce [Grades of Finality (GoF)](#Grades-of-Finality-GoF) where a higher GoF represents a higher confidence.

| Name                 | Component    | Initial/local opinion | Consensus    | Comparable blockchain mechanism for voting/finality |
| -------------------- | --- | --------------------- | ------------ | --------------------------------------------------- |
| voting on conflicts  | UTXO ledger    | OTV/FPCS              | branch/tx AW | longest chain rule                                  |
| finality of messages | Tangle    | inclusion score via tip selection       | message AW   | x block rule                                        |


On an abstract level, a node can be seen as a replicated state machine, just following whatever it receives through the Tangle, and, in case of messages containing transactions, modifying the UTXO ledger. Only when a node wants to issue a message (read as: *cast a vote*) it needs to evaluate its own local opinion via [modular conflict selection function](#Modular-Conflict-Selection-Function). This decoupling of coming to consensus and setting the initial opinion allows for great flexibility and separation of concerns.





## Approval Weight (AW)
Approval weight represents the [weight](#active-consensus-mana) of branches (and messages), similar to the longest chain rule in Nakamoto consensus. However, instead of selecting a leader based on a puzzle (PoW) or stake (PoS), it allows every node to express its opinion by simply issuing any message and attaching it in a part of the Tangle it *likes* (based on its initial opinion on messages and possibly utilizing the [like switch](#Like-Switch) to express its opinion on branches).

It is important to note that tracking of AW for branches and markers/messages is orthogonal. Thus, a message can reach a high AW whereas its contained payload, e.g., a transaction being a double spend, does not reach any AW on branch/UTXO level.

### Detailed Design
Approval weight AW increases because of voters (nodes) that cast votes for branches and messages by means of making statements. This is necessary due to the changing nature of cMana over time, which prevents simply counting the AW per branch or message. Additionally, whenever a node changes its opinion on a conflict, the previous vote needs to be invalidated.

#### Definitions
* **Statement**: A statement is any message issued by a *node*, expressing its opinion and casting a (virtual) vote. It can be objectively ordered by its timestamp, and, if equal, its message ID.
* **Branch voter**: A branch voter is a *node* that issued a statement attaching to a branch, and, thus, voting for it.
* **Marker/message voter**: A marker/message's voter is a *node* that issued a statement directly or indirectly referencing a marker/message, including its issuer.

#### Branches
Tracking voters of [branches](ledgerstate.md#branches) is an effective way of objective virtual voting. It allows nodes to express their opinion simply by attaching a statement to a branch they like (see [like switch](#Like-Switch)). This statement needs to propagate down the branch DAG, adding support to each of the branch's parents. In case a voter changes their opinion, support needs to be revoked from all conflicting branches and their children. Thus, a node can only support one branch of a conflict set.

To make this more clear consider the following example:

[![Branch Voter](/img/protocol_specification/branches.png)](/img/protocol_specification/branches.png)



The green node issued **statement 1** and attached it to the aggregated branch `Branch 1.1 + Branch 4.1.1`. Thus, the green node is a voter of all the aggregated branch's parent branches, which are (from top to bottom) `Branch 4.1.1`, `Branch 1.1`, `Branch 4.1`, `Branch 1`, and `Branch 4`.

Then, the green node issued **statement 2** and attached it to `Branch 4.1.2`. This makes the green node a voter of `Branch 4.1.2`, however, `Branch 4.1.1` is its conflict branch and thus support for `Branch 4.1.1` has to be revoked.

`Branch 4.1`, `Branch 4` are parent branches of `Branch 4.1.2`, which the green node is still supporting. Since `Branch 1.1`, `Branch 1` are not conflicting to either of `Branch 4.1.2`'s parents, the green node remains their voter.

Finally, the green nodes issued **statement 3**, which is in `Branch 2`. Now the green node is a voter of `Branch 2`, and no longer a voter of `Branch 1`, since `Branch 1` is conflicting to `Branch 2`. Note that, this voter removal will propagate to child branches. Thus, the green node is removed from `Branch 1.1` as well.
`Branch 3`, `4` and both of their child branches have nothing to do with this attachement, the voter status remains.

It is important to notice that the arrival order of the statements does not make a difference on the final outcome. Due to the fact that statements can be ordered objectively, every node in the network eventually comes to the same conclusion as to who is supporting which branch, even when nodes change their opinions.


##### Calculation of Approval Weight
The approval weight itself is calculated every time a new voter is added/removed to a branch. The AW for a branch *B* is calculated as follows:

```
AW(B) = 'active cMana of voters(B)' / 'total active cMana'
```

#### Markers
It would be computationally expensive to track the AW for each message individually. Instead, we approximate the AW with the help of [markers](markers.md). Once a marker fulfills a GoF, the corresponding GoF value is propagated into its past cone until all  messages have an equal or higher GoF.

Recall that markers are not part of the core protocol. As such, this description is merely an optimization from an implementation standpoint.

Rather than keeping a list of voters for each marker and collecting voters for each marker (which would also be expensive), we keep a list of voters along with its approved marker index for each marker sequence. This approach provides a simple and fast look-up for marker voters making use of the Tangle structure as mapped by the markers.

For each marker sequence, we keep a map of voter to marker index, meaning a voter supports a marker index `i`. This implies that the voter supports all markers with index `<= i`.

Take the figure below as an example: 
![MarkersApprovalWeight SequenceVoters](/img/protocol_specification/MarkersApprovalWeight.png)

The purple circles represent markers of the same sequence, the numbers are marker indices.

Four nodes (A to D) issue statements with past markers of the purple sequence. Node A and D issue messages having past marker with index 6, thus node A and D are the voters of marker 6 and all markers before, which is 1 to 5. On the other hand, node B issues a message having past marker with index 3, which implies node B is a voter for marker 1 and 2 as well.

This is a fast look-up and avoids walking through a marker's future cone when it comes to retrieving voters for approval weight calculation.

For example, to find all voter of marker 2, we iterate through the map and filter out those support marker with `index >= 2`. In this case, all nodes are its voters. As for marker 5, it has voters node A and D, which fulfill the check: `index >= 5`.

Here is another more complicated example with parent sequences:
![MarkersApprovalWeight SequenceVoters](/img/protocol_specification/MarkersApprovalWeightSequenceVoters.png)

The voter will be propagated to the parent sequence.

Node A issues message A2 having past markers `[1,4], [3,4]`, which implies node A is a voter for marker `[1,1]` to `[1,4]`, `[2,1]` to `[2,3]`, and `[3,4]`  as well as the message with marker `[3,5]` itself.

##### Calculation of Approval Weight
The approval weight itself is calculated every time a new voter is added to a marker. The AW for a marker *M* is calculated as follows:

```
AW(M) = 'active cMana of voters(M)' / 'total active cMana'
```


### Grades of Finality (GoF)
The tracking of AW itself is objective as long as the Tangle converges on all nodes. However, delays, network splits and ongoing attacks might lead to differences in perception so that a finality can only be expressed probabilistically. The higher the AW, the less likely a decision is going to be reversed. To abstract and simplify this concept we introduce the GoF. Currently, they are simply a translation of AW thresholds to a GoF, but one can imagine other factors as well.

**Message / non-conflicting transaction**
GoF | AW
-- | --
0 | < 0.25
1 | >= 0.25
2 | >= 0.45
3 | >= 0.67

**Branch / conflicting transaction**
GoF | AW
-- | --
0 | < 0.25
1 | >= 0.25
2 | >= 0.45
3 | >= 0.67

These thresholds play a curcial role in the safety vs. liveness of the protocol, together with the exact workings of [active cMana](#Active-cMana). We are currently investigating them with in-depth simulations.
* The higher the AW threshold the more voters a branch or message will need to reach a certain GoF -> more secure but higher confirmation time.
* As a consequence of the above point, TangleTime will be tougher to advance; making the cMana window more likely to get stuck and confirmations to halt forever.

An application needs to decide when to consider a message and (conflicting) transaction as *confirmed* based on its safety requirements. Conversely, a message or branch that does not gain enough AW stays pending forever (and is orphaned/removed on snapshotting time).


## Modular Conflict Selection Function
The modular conflict selection function is an abstraction on how a node sets an initial opinion on conflicts. By decoupling the objective perception of AW and a node's initial opinion, we gain flexibility and it becomes effortless to change the way we set initial opinions without modifying anything related to the AW.


### Pure On Tangle Voting (OTV)
The idea of pure OTV is simple: set the initial opinion based on the currently heavier branch as perceived by AW. However, building a coherent overall opinion means that conflicting realities (possible outcomes of overlapping conflict sets) can not be liked at the same time, which makes finding the heaviest branch to like not as trivial as it may seem.

In the examples below, a snapshot at a certain time of a UTXO-DAG with its branches is shown. The gray boxes labelled with `O:X` represent an output and and arrow from an output to a transaction means that the transaction is consuming this output. An arrow from a transaction to an output creates this output. An output being consumed multiple times is a conflict and the transactions create a branch, respectively. The number assiged to a branch, e.g., `Branch A = 0.2`, defines the currently perceived Approval Weight of the branch. A branch highlighted in **bold** is the outcome of applying the pure OTV rules, i.e., the branches that are liked from the perspective of the node.

**Example 1**
The example below shows how applying the heavier branch rule recursively results in the end result of `A`, `C`, `E`, and thus the aggregated branch `C+E` being liked. Looking at the individual branch weights this result might be surprising: branch `B` has a weight of `0.3` which is bigger than its conflict branch `A = 0.2`. However, `B` is also in conflict with `C` which has an even higher weight `0.4`. Thus, `C` is liked, `B` cannot be liked, and `A` suddenly can become liked again.

`E = 0.35` is heavier than `D = 0.15` and is therefore liked. An (aggregated) branch can only be liked if all its parents are liked which is the case with `C+E`.

![OTV example 1](/img/protocol_specification/otv-example-1.png)

**Example 2**
This example is exactly the same as example 1, except that branch `C` has a weight of `0.25` instead of `0.4`. Now the end result is branches `B` and `E` liked. Branch `B` is heavier than branch `C` and `A` (winning in all its conflict sets) and becomes liked. Therefore, neither `A` nor `C` can be liked.


Again, `E = 0.35` is heavier than `D = 0.15` and is therefore liked. An (aggregated) branch can only be liked if all its parents are liked which is not the case with `C+E` in this example.

![OTV example 2](/img/protocol_specification/otv-example-2.png)



### Metastability: OTV and FPCS
Pure OTV is susceptible to metastability attacks: If a powerful attacker can keep any branch of a conflict set reaching a high enough approval weight, the attacker can prevent the network from tipping to a side and thus theoretically halt a decision on the given conflicts indefinitely. Only the decision on the targeted conflicts is affected but the rest of the consensus can continue working. By forcing a conflict to stay unresolved, an attacker can, at most, prevent a node from pruning resources related to the pending decision.

In order to prevent such attacks from happening we are planning to implement FPCS with OTV as a conflict selection function. A more detailed description can be found [here](https://iota.cafe/t/on-tangle-voting-with-fpcs/1218).




## Like Switch
Without the like switch, messages vote for branches simply by attaching in their underlying conflict's future cone. While this principle is simple, it has one major flaw: the part of the Tangle of the losing conflict is abandoned so that only the *valid* part remains. This might lead to mass orphanage of "unlucky" messages that happened to first vote for the losing branch. With the help of weak parents these messages might be *rescued* without a reattachment but the nature of weak parents makes it necessary so that every message needs to be picked up individually. Next to the fact that keeping such a weak tip pool is computationally expensive, it also open up orphanage attack scenarios by keeping conflicts undecided (metastability attack turns into orphanage attack).

The like switch is a special type of parent reference that enables  keeping everything in the Tangle, even conflicting transactions that are not included into the valid ledger state by means of consensus. Therefore, it prevents mass orphanage and enables a decoupling of **voting on conflicts (UTXO ledger)** and **finality of messages / voting on messages (Tangle)**. It makes the overall protocol (and its implementation) not only more efficient but also easier to reason about and allows for lazy evaluation of a node's opinion, namely only when a node wants to issue a message (read as: *cast a vote*).


From a high-level perspective, the like switch can be seen as a set of rules that influence the way a message inherits its branches. Using only strong parents, a message inherits all its parents' branches. A like parent retains all the properties of the strong parent (i.e., inherit the branch of said parent) but additionally it means to exclude all branches that are conflicting with the liked branch.
Through this mechanism, it becomes possible to attach a message anywhere in the Tangle but still only vote for the branches that are liked. Thus, decoupling of message AW and branch AW.

**Examples**
To make this more clear, let's consider the following examples. The branches `A` and `B`, as well as `C` and `D` form an independent conflict set, respectively. The `Branch Weights` are the weights as perceived by the node that currently wants to create a message.


**Message creation**
A node performs random tip selection (e.g. URTS) and in this example selects messages `5` and `11` as strong parents. Now the node needs to determine whether it currently *likes* all the branches of the selected messages (`red, yellow, green`) in order to apply the like switch (if necessary) and vote for its liked branches.

![Like switch: message creation undecided](/img/protocol_specification/like-switch-message-creation-1.png)

When performing the conflict selection function with pure OTV it will yield the result:
- `red` is disliked, instead like `purple`
- `green` is disliked, instead like `yellow`

Therefore, the message needs to set two like references to the messages that introduced the branch (first attachment of the transaction). The final result is the following:
- branches `purple` and `yellow` are supported
- message `5` (and its entire past cone, `3`, `2`, `1`) is supported
- message `11` (and its entire past cone, `6`, `4`, `2`, `1`) is supported
- message `6` (and its entire past cone, `4`, `2`, `1`)
- message `7` (and its entire past cone, `1`)

![Like switch: message creation](/img/protocol_specification/like-switch-message-creation-2.png)


**Message booking**
On the flip side of message creation (casting a vote) is applying a vote when booking a message, where the process is essentiall just reversed. Assuming a node receives message `17`. First, it determines the branches of the strong parents (`red`, `yellow`, `purple`) and like parents (`red`). Now, it removes all branches that are conflicting with the like parents' branches (i.e., `purple`) and is left with the branches `red` and `yellow` (and adds the like branches to it, which in this case is without effect). If the resulting branch is `invalid`, (e.g., because it combines conflicting branches) then the message itself is considered invalid because the issuer did not follow the protocol.

In this example the final result is the following (message `17` supports):
- branches `red` and `yellow`
- message `16` and its entire past cone
- message `11` and its entire past cone
- message `4` and its entire past cone

![Like switch: message booking](/img/protocol_specification/like-switch-message-booking.png)

## Active cMana
The consensus mechanism weighs votes on branches (messages in future cone with like switch) or message inclusion (all messages in future cone) by the limited resource cMana, which is thus our Sybil-protection mechanism. cMana can be pledged to any nodeID, including offline or non-existing nodes, when transferring funds (in the proportion of the funds) and is instantly available (current implementation without EMA). Funds might get lost/locked over time, and, therefore, the total accessible cMana declines as well. **Consequently, a fixed cMana threshold cannot be used to determine a voting outcome.**

Finalization of voting outcomes should happen once a conflict is *sufficiently* decided and/or a message is *sufficiently* deep in the Tangle. However, measuring AW in terms of cMana alone does not yield enough information. Therefore, a degree/grade of finalization and of voting outcome in relation to *something*, e.g., **recently active nodes**, is preferred. This measure should have the following properties:
- security/resilience against various attacks
- lose influence (shortly) after losing access to funds
- no possibility of long-range attacks
- no too quick fluctuations
- real incentives


### Current Implementation
Active cMana in GoShimmer basically combines two components in an active cMana WeightProvider: the TangleTime and the current state of cMana. A node is considered to be active if it has issued any message in the last `activeTimeThreshold=30min` with respect to the TangleTime. The total active consensus mana is, therefore, the sum of all the consensus mana of each active node.

#### TangleTime
The TangleTime is the issuing time of the last confirmed message. It cannot be attacked without controlling enough mana to accept incorrect timestamps, making it a reliable, attack-resistant quantity.

![Tangle Time](/img/protocol_specification/tangle_time.jpg)

#### cMana
The current state of cMana is simply the current cMana vector, at the time the active cMana is requested.


#### Putting it together
Every node keeps track of a list of active nodes locally. Whenever a node issues a message it is added to the list of active nodes (nodeID -> issuing time of latest message). When the active cMana is requested all relevant node weights are returned. Relevant here means the following:
- the node has more than `minimumManaThreshold=0` cMana to prevent bloating attacks with too little cMana
- there is a message that fulfills the condition `issuing time <= TangleTime && TangleTime - issuing time <= activeTimeThreshold` where `activeTimeThreshold=30min` (see the following example, messages `1` and `3` are not within the window)

![Active cMana window](/img/protocol_specification/active-cMana-window.png)


### Example
When syncing (`TT=t0`) and booking a message from time `t1`, active cMana is considered from `t0-activeTimeThreshold`. Once this message gets confirmed, the TangleTime advances to `TT=t1`. For the next message at `t2`, `TT=t1-activeTimeThreshold` will be considered. Using active cMana in this way, we basically get a sliding window of how the Tangle emerged and *replay* it from the past to the present.


### Pros
- replaying the Tangle as it emerged
- always use cMana from the current perspective
- relatively simple concept

### Cons
- active cMana does not yield sufficient information (e.g. when eclipsed), it might look like something is 100% confirmed even though only 2% of the total cMana are considered active.
- active cMana might change quickly nodes with high mana suddenly become active
- if nodes are only able to issue messages when "in sync" and no message gets confirmed within that time, nobody might be able to issue messages anymore
- if a majority/all active cMana nodes go offline _within the active cMana window_, consensus will halt forever because the TangleTime can never advance unless a majority of these nodes move the TangleTime forward

This reflects the current implementation and we are currently investigating active cMana with in-depth simulations to improve the mechanism.