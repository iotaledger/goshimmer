---
description: Markers is a tool to efficiently estimate the approval weight of a block and that reduces the portion of the Tangle that needs to be traversed, and which finally results in the confirmation state.
image: /img/protocol_specification/example_1.png
keywords:
- approval weight
- marker
- block
- sequence
- future marker
- new marker
- part marker
- past cone
---
# Markers

## Summary

Operations that involve traversing the Tangle are very performance intensive and, thus, we need to minimize the amount of traversing to keep algorithms fast. Markers are a tool to infer structural knowledge about the Tangle without actually traversing it.

:::info Note

**Markers** are not a core module of the Coordicide project.

:::

## Motivation

*Markers* are a tool to infer knowledge about the structure of the Tangle, therefore, we use them to keep algorithms fast. Specifically, markers are used for:
+ past/future cone membership;
+ approximate approval weight of any block;
+ tagging sections of the Tangle (e.g., conflicts) without having to traverse each block individually.

## Definitions
Let's define the terms related to markers:
* **Sequence:** A sequence is a chain of markers where each progressing marker contains all preceding markers of the sequence in its past cone.
* **Sequence Identifier (`SID`):** A Sequence Identifier is the unique identifier of a Sequence.
* **Marker Index (`MI`):** A Marker Index is the marker rank in the marker DAG. Throughout the code the marker rank will be called index.
* **marker:** A marker is a pair of numbers: `SID` and `MI` associated to a given block. Markers carrying the same `SID` belong to the same Sequence.
* **future marker (`FM`):** A future marker of a block is the first marker in its future cone from different sequences.
* **past marker (`PM`):** A past marker of a block is a marker in its past cone (can be multiple markers of distinct sequences). For a given sequence it is set to the newest past marker of its parents, that is the one that has the largest `MI`. The past marker of a marker is set to itself.


## Design
On a high level, markers provide structural knowledge of the Tangle and each individual block without the need to traverse (aka walking the Tangle). Markers are a form of meta-information (for each block) that each node locally creates when processing blocks. They can be seen as specific, uniquely tainted blocks that, taken together, again build a DAG within the Tangle. We can then utilize this marker DAG to determine structural details.

![](https://i.imgur.com/3x7H68t.png)



The above example shows a Tangle with the red blocks being markers in the same sequence (more details on sequences later). A marker is uniquely identified by `sequenceID,index`, where the index is ever-increasing. Any block can be "selected" as a marker if it fulfills a certain set of rules:
- every n-th block (in the example, each block is tried to be set as a marker)
- latest marker of sequence is in its past cone.

The markers build a chain/DAG and because of the rules it becomes clear that `marker 0,1` is in the past cone of `marker 0,5`. Since markers represent meta-information for the underlying blocks and each block keeps the latest marker in its past cone as *structural information*, we can infer that `block B` (`FM 0,2`) is in the past cone of `block I` (`PM 0,3`)  Similarly, it is evident that `block D` is in the past cone of `block J`.



### Sequences
A sequence is a chain of markers where each progressing marker contains all preceding markers of the sequence in its past cone. However, this very definition entails a problem: what if there are certain parts of the Tangle that are disparate to each other. Assuming only a single sequence, this would mean that a certain part of the Tangle can't get any markers. In turn, certain operations within this part of the Tangle would involve walking.

For this reason, we keep track of the *marker distance*, which signals the distance of blocks in the Tangle in a certain past cone where no marker could be assigned. If this distance gets too big, a new sequence is created as is shown in the example below (marker distance to spawn a new sequence = 3).


![](https://i.imgur.com/Q44XZgk.png)



The example above shows a side chain starting from `block L` to `block P` where it merges back with the "main Tangle". There can be no new marker assigned as none of the `blocks L-O` have the latest marker of `sequence 0` in their past cone. The marker distance grows and eventually a marker is created at `block N`. Following, a marker can be assigned to `block O` and `block P`. The latter is special because it combines two sequences. This is to be expected as disparate parts of the Tangle should be merged eventually. In case a block has markers from multiple sequences in its past cones the following rules apply:
- Assign a marker in the highest sequence if possible. If not possible, try to assign a marker in the next lower sequence.
- The index is `max(marker1.Index,marker2.Index,...)`

With these rules in mind, it becomes clear why `block P` has the `marker 1,6` and `block R` has `marker 1,7`. In case of `block Q`, no marker can be assigned to `sequence 1`, and, thus, a new marker in `sequence 0` is created.

Always continuing the highest seqeuence should result in smaller sequences being discontinued once disparate parts of the Tangle merge and overall a relatively small number of sequences (optimally just one) is expected to be active at any given moment in time.


### Sequence Graph
The information that markers yield about past and future cone is only valid for any given sequence individually. However, to relate markers of separate sequences, we need to track dependencies between sequences.
Therefore, sequences build a graph between each other, where relationships between the sequences can be seen.

Each sequence keeps track of **referenced sequences** and **referencing sequences** at a specific marker index so that bidirectional traversing into the future and past are possible from a sequence is possible.

Specifically, in our example there are 3 bidirectional references between `sequence 0` and `sequence 1`.
Sequence 0:
- `0,1`<->`1,2`
- `0,5`<->`1,6`
- `0,6`<->`1,7`

Sequence 1:
- `1,2`<->`0,1`
- `1,6`<->`0,5`
- `1,7`<->`0,6`


![](https://i.imgur.com/EhbJohc.png)



## Usage

### Markers Application: Approval Weight Estimation
To approximate the approval weight of a block, we simply retrieve the approval weight of its `FM` list. Since the block is in the past cone of its `FM`s, the approval weight and the finality will be at least the same as its `FM`s. This will of course be a lower bound (which is the “safe” bound), but if the markers are set frequently enough, it should be a good approximation.
In practice, we propagate the GoF finality to blocks in a marker's past cone until we reach another marker.

For details of managing approval weight of each marker and approval weight calculation thereof please refer to [Approval Weight](consensus_mechanism.md#approval-weight-aw).


### Conflict Mapping
Conflicts are introduced to the Tangle when double spends occur and are carried forward (inherited) by blocks until a conflict is resolved (merge to master). As such, each block needs to carry conflict information and if a conflict arises deep within the Tangle, each block would need to be traversed individually, which makes this operation very expensive and thus attackable.

Therefore, we utilize markers to store conflict information for blocks and store only a **difference** of conflicts (subtracted/added) on each block individually. In that way, propagation of conflicts can happen via structural marker information and not every block needs to be updated. When querying conflict information of a block, first all conflicts of the block's past markers are retrieved and then combined with the diff of the block itself to result in the block's overall conflict.
