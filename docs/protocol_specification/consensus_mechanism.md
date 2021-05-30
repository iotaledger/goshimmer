# Consensus Mechanism

- workings together with FCoB (opinion setting) / FPC (pre-consensus, metastability breaker)
- leaderless consensus

## FCoB

## FPC

## Approval Weight (AW)
Approval weight represents the [weight](#active-consensus-mana) of branches (and messages), similar to the longest chain rule in Nakamoto consensus. However, instead of selecting a leader based on a puzzle (PoW) or stake (PoS), it allows every node to express its opinion by simply issuing any message and attaching it in a part of the Tangle it *likes* (based on FCoB/FPC). This process is also known as virtual voting, and has been previously described as [On Tangle Voting](https://medium.com/@hans_94488/a-new-consensus-the-tangle-multiverse-part-1-da4cb2a69772). 

If a node realizes its opinion according to FCoB/FPC differs from that of the majority of weight, it has to do a reorg of its perception according to the heavier branch. In that way, all nodes will eventually converge to the heaviest branches, and, thus, resolve conflicts efficiently. 

AW also serves as a probabilistic finality tool for individual messages and their payloads, i.e., transactions.

### Finalization
TODO: add finality for branches?

Finality must always be considered as a probabilistic finality in the sense that a message is included in the ledger with a very high probability. Two qualities desired from a finality criteria are fast confirmation rate and a high probability of non-reversibility. 
A message is considered finalized or confirmed if one of the following holds:

1. Its approval weight is higher than *0.5*, and it belongs to the *MasterBranch*.
2. It belongs to a branch *B* with conflicting branches, but the approval weight of any of those conflicting branches is at least *0.5* lower than *B*.

Conversely, a message that does not gather enough AW will not be finalized, and, thus, will be pending until it might be orphaned if not reachable via current tips anymore.

### Detailed Design
Approval weight is tracked with the help of supporters that cast votes for branches and messages by means of making statements. This is necessary due to the changing nature of cMana over time, which prevents simply counting the AW per branch or message. 

#### Definitions
* **Statement**: A statement is any message issued by a *node*, expressing its opinion and casting a (virtual) vote. It can be objectively ordered by its timestamp, and, if equal, its message ID.
* **Branch supporter**: A branch supporter is a *node* that issued a statement attaching to a branch, and, thus, voting for it.
* **Marker/message supporter**: A marker/message's supporter is a *node* that issued a statement directly or indirectly referencing a marker/message, including its issuer.

#### Branches
Tracking supporters of branches and following the heavier branch effectively is On Tangle Voting. It allows nodes to express their opinion simply by attaching a statement to a branch they like. This statement needs to propagate down the branch DAG, adding support to each of the branch parents. In case a supporter changes their opinion, support needs to be revoked from all conflicting branches and their children. Thus, a node can only support one branch of a conflict set. 

To make this more clear consider the following example:
![Branch Supporter](https://user-images.githubusercontent.com/11289354/112409357-518e9480-8d54-11eb-8a40-19f4ab33ea35.png)

The green node issued **statement 1** and attached it to the aggregated branch `Branch 1.1 + Branch 4.1.1`. Thus, the green node is a supporter of all the aggregated branch's parent branches, which are (from top to bottom) `Branch 4.1.1`, `Branch 1.1`, `Branch 4.1`, `Branch 1`, and `Branch 4`.

Then, the green node issued **statement 2** and attached it to `Branch 4.1.2`. This makes the green node a supporter of `Branch 4.1.2`, however, `Branch 4.1.1` is its conflict branch and thus support for `Branch 4.1.1` has to be revoked.

`Branch 4.1`, `Branch 4` are parent branches of `Branch 4.1.2`, which the green node is still supporting. Since `Branch 1.1`, `Branch 1` are not conflicting to either of `Branch 4.1.2`'s parents, the green node remains their supporter.

Finally, the green nodes issued **statement 3**, which is in `Branch 2`. Now the green node is a supporter of `Branch 2`, and no longer a supporter of `Branch 1`, since `Branch 1` is conflicting to `Branch 2`. Note that, this supporter removal will propagate to child branches. Thus, the green node is removed from `Branch 1.1` as well.
`Branch 3`, `4` and both of their child branches have nothing to do with this attachement, the supporter status remains.

It is important to notice that the arrival order of the statements does not make a difference on the final outcome. Due to the fact that statements can be ordered objectively, every node in the network eventually comes to the same conclusion as to who is supporting which branch, even when nodes change their opinion.


##### Calculation of Approval Weight
The approval weight itself is calculated every time a new supporter is added to a branch. The AW for a branch *B* is calculated as follows:

```
AW(B) = supporters(B) dot 'active cMana nodes' / 'total active cMana'
```

It is then evaluated whether it fulfills the [finalization](#finalization) criterion. If so, the branch is set to *confirmed*, while all its conflicts are set to *rejected*.

**Reorg**: In case the node confirmed another branch of the conflict set first, e.g., because of a difference in perception of the ledger state, it will have to do reorg. This means, the node needs to adjust its perception of the ledger state, so that, eventually, all nodes converge and follow the heaviest branch by active cMana.

#### Markers
It would be computationally expensive to track the AW for each message individually. Instead, we approximate the AW with the help of [markers](003-markers.md). Once a marker fulfills the [finalization](#finalization) criterion, the confirmation is propagated into its past cone until all the messages are confirmed.

Rather than keeping a list of supporters for each marker and collecting supporters for each marker (which would also be expensive), we keep a list of supporters along with its approved marker index for each marker sequence. This approach provides a simple and fast look-up for marker supporters making use of the Tangle structure as mapped by the markers.

For each marker sequence, we keep a map of supporter to marker index, meaning a supporter supports a marker index `i`. This implies that the supporter supports all markers with index `<= i`.

Take the figure below as an example:
![MarkersApprovalWeight SequenceSupporters-Page-2](https://user-images.githubusercontent.com/11289354/112416694-21012780-8d61-11eb-8089-cb9f5b236f30.png)

The purple circles represent markers of the same sequence, the numbers are marker indices.

Four nodes (A to D) issue statements with past markers of the purple sequence. Node A and D issue messages having past marker with index 6, thus node A and D are the supporters of marker 6 and all markers before, which is 1 to 5. On the other hand, node B issues a message having past marker with index 3, which implies node B is a supporter for marker 1 and 2 as well.

This is a fast look-up and avoids walking through a marker's future cone when it comes to retrieving supporters for approval weight calculation.

For example, to find all supporter of marker 2, we iterate through the map and filter out those support marker with `index >= 2`. In this case, all nodes are its supporters. As for marker 5, it has supporters node A and D, which fulfill the check: `index >= 5`.

Here is another more complicated example with parent sequences:
![MarkersApprovalWeight SequenceSupporters-Page-2(1)](https://user-images.githubusercontent.com/11289354/112433680-8cf18900-8d7d-11eb-8944-54030581a033.png)

The supporter will be propagated to the parent sequence.

Node A issues message A2 having past markers `[1,4], [3,5]`, which implies node A is a supporter for marker `[1,1]` to `[1,4]`, `[2,1]` to `[2,3]`, and `[3,4], [3,5]` as well.

##### Calculation of Approval Weight
The approval weight itself is calculated every time a new supporter is added to a marker, and the marker's branch *B* has reached its finality criterion. The AW for a marker *M* is calculated as follows:

```
AW(M) = supporters(B) dot supporters(M) dot 'active cMana nodes' / 'total active cMana'
```

It is then evaluated whether it fulfills the [finalization](#finalization) criterion. If so, the marker's message is set to *confirmed* as well as all messages in its past cone.


## Active Consensus Mana?
