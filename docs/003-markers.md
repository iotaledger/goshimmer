# Markers
## Summary
In order to know whether a message in the Tangle is orphaned or not, we introduce **grades of finality** to interpret the status of a message. The higher grade of finality is determined by the **approval weight**, which is the proportion of active consensus mana approving a given message.

In order to compute the approval weight of a given message  we need to traverse the Tangle from the message to the tips and sum up the active consensus mana of all the messages in its future cone. A **marker** is a tool to efficiently estimate the approval weight of a message and that reduces the portion of the Tangle we need to walk, and which finally results in the grade of finality.

**Note**: *Markers* is not a core module of the Coordicide project.

## Motivation
*Markers* is a tool to infer knowledge about the structure of the Tangle in terms of:
+ past/future cone membership;
+ approximate approval weight of any message;
+ tagging sections of the Tangle (e.g., branches) without having to traverse each message individually.

## Dependency
Active Consensus Mana


## Detailed Design

Let's define the terms related to markers:
* **Sequence:** A Sequence is a sequence of markers. Each Sequence corresponds to a UTXO branch, which help us to track the structure independently. 
* **Sequence Identifier (SI):** A Sequence Identifier is the unique identifier of a Sequence.
* **Marker Index (MI):** A Marker Index is the marker rank in the marker DAG. Throughout the code the marker rank will be called index. 
* **marker:** A marker is a pair of numbers: SI and MI associated to a given message. Markers carrying the same SI belong to the same Sequence.
* **future marker (FM):** A future marker of a message is the first marker in its future cone from different sequences; this field in the message metadata is updated when the new marker is generated in the future, following the rules defined in (#future-markers).
* **past marker (PM):** A past marker of a message is a marker in its past cone. For a given sequence it is set to the newest past marker of its parents, that is the one that has the largest MI. The past marker of a marker is set to itself.
* **sequence rank:** The rank of a sequence will be simply called rank throughout this code. Bear in mind that for clarity the marker rank is called index.


### The Sequence
Sequences are used to track the UTXO DAG branches, each branch corresponds to a sequence with a unique $SI$, and the sequences form a DAG as well.

#### Sequence Structure

<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>id</td>
        <td>uint64</td>
        <td>The sequence identifier of the sequence.</td>
    </tr>
    <tr>
        <td>parentReferences</td>
        <td>map[uint64]Thresholdmap</td>
        <td>The marker referenced map of each parent marker.</td>
    </tr>
    <tr>
        <td>rank</td>
        <td>uint64</td>
        <td>The rank of the sequence in the marker DAG.</td>
    </tr>
    <tr>
        <td>highestIndex</td>
        <td>uint64</td>
        <td>The highest MI of the marker sequence.</td>
    </tr>
    <tr>
        <td>lowestIndex</td>
        <td>uint64</td>
        <td>The lowest MI of the sequence.</td>
    </tr>
</table>


#### Create Sequence
A new sequence is created when:
1. there's a conflict in a UTXO branch.
2. the UTXO branches are aggregated.
3. UTXO branches are merged.

Each new sequence starts from a new marker.

### The Markers
Markers are messages selected from the tip set periodically and assigned unique identifiers, in the form of $[SI, MI]$. 

#### Marker Structure
<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>SequenceID</td>
        <td>uint64</td>
        <td>The Sequence identifier of the marker.</td>
    </tr>
    <tr>
        <td>Index</td>
        <td>uint64</td>
        <td>The index of the marker in the sequence.</td>
    </tr>
</table>

#### Create Markers
A new marker is created when:
1. the default conditions are met, which will be one of these options:
    * **every x messsages**;
    + **every t seconds**;
    + a mix of the first two!
        + Upperbound given by the messages
        + Lower temporal bound given by the time
    + every x messages that reference (directly or indirectly) the previous marker
        + Lower bound given by rank (e.g., how far you are in terms of steps) -> >= 10 or something
        + Upper bound given by the amount of messages referencing the previous one -> ~ 200 msgs
2. A new sequence is created. 
> :mega: to be confirmed here.
 
A new marker is selected from the strong tips set randomly, and selected from the weak tips set if there's no strong tip. A new pair of $[SI, MI]$ is assigned to the new marker. 
> :mega:  to be confirmed here.


The $SI$ is set according to the following rules:
* Inherit the SI from parents if the new marker references the latest marker of a sequence and meets the requirement to set up a new marker without initiating a new MS.
* Create a new SI if it is the first marker of a new sequence.

The $MI$ is set to $MI = 1+ max(referencedMI)$, which complies to the rule:
+ Marker indexes ($MIs$) are monotonically increasing such that $\forall x \in fc(y)$ => $MI_x > MI_y$, where $fc(y)$ is the future cone of $y$ and $x$ is any message in that future cone.

### Past Markers and Future Markers
Each message keeps its associated marker information in two lists:
* past markers 
* future markers 

These lists for past markers and future markers are used to determine whether a message is in the past cone of the other, and the list for future markers also helps us to efficiently estimate the approval weight of a message.

#### StructureDetails Structure
StructureDetails is a structure that will be in the message metadata containing marker information.

<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>Rank</td>
        <td>uint64</td>
        <td>The rank of the message.</td>
    </tr>
    <tr>
        <td>IsPastMarker</td>
        <td>bool</td>
        <td>A flag to indicate whether a message is a marker.</td>
    </tr>
    <tr>
        <td>PastMarkers</td>
        <td>map[SequenceID]Index</td>
        <td><b>PM list</b>, a list of PMs from different sequences.</td>
    </tr>
    <tr>
        <td>FutureMarkers</td>
        <td>map[SequenceID]Index</td>
        <td><b>FM list</b>, a list of FMs from different sequences.</td>     
    </tr>
</table>

##### Past Markers
* The PM list of a marker contains the marker itself only.
* The PM list of non-marker messages is inherited from its **strong** parents, with 2 steps:
    1. for a given sequence select only the nearest marker (i.e. the markers with the highest MI). Thus for every sequence from the parents there will be exactly one marker.
    2. remove those that have been referenced by other markers from this set. 

##### Future Markers
The FM list of a message is empty at start and gets updated when a new marker directly or indirectly references it. The propagation of a FM to its past cone (i.e. the update of the `FutureMarkers` field in the encountered messages) does not continue beyond a message if:
1. the FM list of a message includes a previous marker of the same sequence;
2. the message is the marker in the different sequence, we update the FM list of that marker only.

The figure below shows an example of applying markers to the Tangle with 1 sequence only. The yellow messages are markers with identifiers: $[0,1]$ and $[0,2]$. Both markers are in Sequence $0$ with $MI$ $1$ and $2$ respectively.

For a more complete but complex example, including multiple sequences refer to the next figure.
![](https://i.imgur.com/RluZWCJ.png)


### Markers and Approval Weight
To approximate the approval weight of a message, we simply retrieve the approval weight of its FM list. Since the message is in the past cone of its FMs, the approval weight and the finality will be at least the same as its FMs. This will of course be a lower bound (which is the “safe” bound), but if the markers are set frequently enough, it should be a good approximation.

### Marker Management
Messages can have markers from different sequences in PM list and FM list, the order and referenced relationship among sequences are important for example when it comes to inheriting the PM list from parents. Thus, we need to track these sequences.

Here is an example of how the markers and sequences structures would look in the Tangle:
The purple colored messages are markers.

![](https://i.imgur.com/GENej3O.png)


#### Sequences
For whatever reason a sequence is created, we assign a new $SI = 1+max(referenceSequencesIdentifiers)$. To prevent assigning a new $SI$ when combining same sequences again, we build parents-child relation in a map if a new sequence is created. 

#### Sequence Rank
The rank of a sequence graph is the number of sequences from the starting point to itself. The sequence ranks are shown in the figure above.

When a new sequence is created we check the parent marker' sequences with the function `normalizeMarkers()` in order from high to low rank. In this function, we remove those PMs that it's belonging sequence is referenced by others.

An example is **msg 10** in the figure, $[0,2], [1,1], [2,3]$ are PMs to be considered to inherit. $[2,3]$ is the first marker to check, since it has the highest sequence rank. We select the parent sequences of $[2,3]$, which are $0$ and $1$, and the referenced PMs therein. Next any PMs that are already referenced can be removed. This results in that the PMs of **msg 10** is $[2,3]$ only.

In the following we show an implementation of  `normalizeMarkers()`, which returns the markers and sequences that will be inherited from a message.
```go
// normalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *Manager) normalizeMarkers(markers *Markers) (normalizedMarkersByRank *markersByRank, normalizedSequences SequenceIDs) {
	rankOfSequencesCache := make(map[SequenceID]uint64)

	normalizedMarkersByRank = newMarkersByRank()
	normalizedSequences = make(SequenceIDs)
	// group markers with same sequence rank
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		normalizedSequences[sequenceID] = types.Void
		normalizedMarkersByRank.Add(m.rankOfSequence(sequenceID, rankOfSequencesCache), sequenceID, index)

		return true
	})
	markersToIterate := normalizedMarkersByRank.Clone()

	//iterate from highest sequence rank to lowest
	for i := markersToIterate.HighestRank() + 1; i > normalizedMarkersByRank.LowestRank(); i-- {
		currentRank := i - 1
		markersByRank, rankExists := markersToIterate.Markers(currentRank)
		if !rankExists {
			continue
		}

		// for each marker from the current sequence rank check if we can remove a marker in normalizedMarkersByRank,
		// and add the parent markers to markersToIterate if necessary
		if !markersByRank.ForEach(func(sequenceID SequenceID, index Index) bool {
			if currentRank <= normalizedMarkersByRank.LowestRank() {
				return false
			}

			if !(&CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}).Consume(func(sequence *Sequence) {
				// for each of the parentMarkers of this particular index
				sequence.HighestReferencedParentMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
					// of this marker delete the referenced sequences since they are no sequence tips anymore in the sequence DAG
					delete(normalizedSequences, referencedSequenceID)

					rankOfReferencedSequence := m.rankOfSequence(referencedSequenceID, rankOfSequencesCache)
					// check whether there is a marker in normalizedMarkersByRank that is from the same sequence
					if index, indexExists := normalizedMarkersByRank.Index(rankOfReferencedSequence, referencedSequenceID); indexExists {
						if referencedIndex >= index {
							// this referencedParentMarker is from the same sequence as a marker in the list but with higher index - hence remove the index from the Marker list
							normalizedMarkersByRank.Delete(rankOfReferencedSequence, referencedSequenceID)

							// if rankOfReferencedSequence is already the lowest rank of the original markers list,
							// no need to add it since parents of the referencedMarker cannot delete any further elements from the list
							if rankOfReferencedSequence > normalizedMarkersByRank.LowestRank() {
								markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
							}
						}

						return true
					}

					// if rankOfReferencedSequence is already the lowest rank of the original markers list,
					// no need to add it since parents of the referencedMarker cannot delete any further elements from the list
					if rankOfReferencedSequence > normalizedMarkersByRank.LowestRank() {
						markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
					}

					return true
				})
			}) {
				panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
			}

			return true
		}) {
			return
		}
	}

	return
}
```

### Markers Usages: Past Cone Check
By comparing the past and future markers of messages, we can easily tell if one is in another's past cone. The function returns a `TriBool` representing the three possible statuses: `True`, `False` and `Maybe`. If `Maybe` is returned, then we need to walk the Tangle.

#### Algorithm
```go
// IsInPastCone checks if the earlier Markers are directly or indirectly referenced by the later Markers.
func (m *Manager) IsInPastCone(earlierMarkers *MarkersPair, laterMarkers *MarkersPair) (referenced TriBool) {
	// fast check: if earlier Markers have larger highest Indexes they can't be in the past cone
	if earlierMarkers.PastMarkers.HighestIndex() > laterMarkers.PastMarkers.HighestIndex() {
		return False
	}

	// fast check: if earlier Marker is a past Marker and the later ones reference it we can return early
	if earlierMarkers.IsPastMarker {
		earlierMarker := earlierMarkers.PastMarkers.FirstMarker()
		if earlierMarker == nil {
			panic("failed to retrieve Marker")
		}

		if laterIndex, sequenceExists := laterMarkers.PastMarkers.Get(earlierMarker.sequenceID); sequenceExists {
			if laterIndex >= earlierMarker.index {
				return True
			}

			return False
		}

		if laterMarkers.PastMarkers.HighestIndex() <= earlierMarker.index {
			return False
		}
	}

	if laterMarkers.IsPastMarker {
		laterMarker := laterMarkers.PastMarkers.FirstMarker()
		if laterMarker == nil {
			panic("failed to retrieve Marker")
		}

		// if the earlier Marker inherited an Index of the same Sequence that is higher than the later we return false
		if earlierIndex, sequenceExists := earlierMarkers.PastMarkers.Get(laterMarker.sequenceID); sequenceExists && earlierIndex >= laterMarker.index {
			return False
		}

		// if the earlier Markers are referenced by a Marker of the same Sequence that is larger, we are not in the past cone
		if earlierFutureIndex, earlierFutureIndexExists := earlierMarkers.FutureMarkers.Get(laterMarker.sequenceID); earlierFutureIndexExists && earlierFutureIndex > laterMarker.index {
			return False
		}

		// if the earlier Markers were referenced by the same or a higher future Marker we are not in the past cone
		// (otherwise we would be the future marker)
		if !laterMarkers.FutureMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
			earlierIndex, similarSequenceExists := earlierMarkers.FutureMarkers.Get(sequenceID)
			return !similarSequenceExists || earlierIndex < laterIndex
		}) {
			return False
		}

		if earlierMarkers.PastMarkers.HighestIndex() >= laterMarker.index {
			return False
		}
	}

	// if the highest Indexes of both past Markers are the same ...
	if earlierMarkers.PastMarkers.HighestIndex() == laterMarkers.PastMarkers.HighestIndex() {
		// ... then the later Markers should contain exact copies of all of the highest earlier Markers because parent
		// Markers get inherited and if they would have been captured by a new Marker in between then the highest
		// Indexes would no longer be the same
		if !earlierMarkers.PastMarkers.ForEach(func(sequenceID SequenceID, earlierIndex Index) bool {
			if earlierIndex == earlierMarkers.PastMarkers.HighestIndex() {
				laterIndex, sequenceExists := laterMarkers.PastMarkers.Get(sequenceID)
				return sequenceExists && laterIndex != earlierIndex
			}

			return true
		}) {
			return False
		}
	}

	if earlierMarkers.FutureMarkers.HighestIndex() == laterMarkers.FutureMarkers.HighestIndex() && false {
		// the earlier future markers need to contain all later ones because if there would be another marker in between that shadows them the later future Marker would have a higher index
		if !laterMarkers.FutureMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
			if laterIndex == laterMarkers.FutureMarkers.highestIndex {
				earlierIndex, sequenceExists := earlierMarkers.FutureMarkers.Get(sequenceID)
				return sequenceExists && earlierIndex == laterIndex
			}

			return true
		}) {
			return False
		}
	}

	// detailed check: earlier marker is referenced by something that the later one references
	if m.markersReferenceMarkers(laterMarkers.PastMarkers, earlierMarkers.FutureMarkers, false) {
		return True
	}

	// detailed check: the
	if m.markersReferenceMarkers(earlierMarkers.FutureMarkers, laterMarkers.PastMarkers, true) {
		return Maybe
	}

	return False
}

```

### Markers Usages: Approval Weight Estimation
The approval weight of every marker is easy to compute (and hence its finality too): to check the weight of marker $i$, the node looks at the tip list $X$, and computes the proportion of messages in $X$ whose MRRM is $i$ or greater.

Store the approval weight
* save the nodeID per marker, and do the union when we want the approval weight -> we don't want to count the mana twice
* bitmask, we always have a fixed list of consensus mana of each epoch, we could use a bitmask to represent that.
