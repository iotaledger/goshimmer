# Markers
## Summary
To know a message in the Tangle is orphaned or not, we introduce **grades of finality** to interpret the status of a message. However, the higher grades of finality is determined by the **approval weight**, which is the proportion of active consensus mana approving a given message.

Computing the approaval weight of a given message needs to traverse the Tangle from the message to the tips and sum up the active consensus mana of all the messages in its future cone. And **marker** is a tool to efficiently estimate the approval weight of a message that reduces the portion of the Tangle we need to walk,  and finally results in the grade of finality.

**Note**: Markers is not a core module of the Coordicide project.

## Motivation
Markers is a tool to infer knowledge about the structure of the Tangle in terms of:
+ past/future cone membership;
+ approximate approval weight of any message;
+ tagging sections of the Tangle (e.g., branches) without having to traverse each message individually.

## Dependency
Active Consensus Mana

## Detailed Design

Let's define the terms related to markers:
* **Marker Sequence:** A Marker Sequence is a sequence of markers with the same Marker Sequence Identifier. Each Marker Sequence corresponds to a UTXO branch, which help us to track the structure independently.
* **Marker Sequence Identifier (MSI):** A Marker Sequence Identifier is the unique identifier of a Marker Sequence.
* **Marker Index (MI):** A Marker Index is the index of the marker in a Marker Sequence
* **marker:** A marker is a pair of numbers: MSI and MI associated to a given message. Markers carrying the same MSI belong to the same Marker Sequence.
* **future marker (FM):** A future marker of a message is the nearest marker in its future cone; this field is updated when the new marker is generated. The future marker has to be the same branch as the message.
* **past marker (PM):** A past marker of a message is the marker in its past cone. It is set to the newest past marker of its parents, that is the one that has the largest MI. The past marker of a marker is set to itself.

### The Marker Sequence
Marker sequences are used to track the UTXO DAG branches, each branch corresponds to a marker sequence with a unique $MSI$. 

#### Marker Sequence Structure

<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>id</td>
        <td>uint64</td>
        <td>The marker sequence identifier of the marker sequence.</td>
    </tr>
    <tr>
        <td>parentReferences</td>
        <td>map[uint64]Thresholdmap</td>
        <td>The marker referenced map of each parent marker.</td>
    </tr>
    <tr>
        <td>rank</td>
        <td>uint64</td>
        <td>The rank of the marker sequence.</td>
    </tr>
    <tr>
        <td>highestIndex</td>
        <td>uint64</td>
        <td>The highest MI of the marker sequence.</td>
    </tr>
    <tr>
        <td>lowestIndex</td>
        <td>uint64</td>
        <td>The lowest MI of the marker sequence.</td>
    </tr>
</table>

#### Create Marker Sequence
A new marker sequence is created when:
1. there's a conflict in UTXO branch.
2. the UTXO branches are aggregated.
3. UTXO branches are merged.

Each new marker sequence starts from a new marker.

### The Markers
Markers are messages selected from the tip set periodically and assigned unique identifiers, in the form of $[MSI, MI]$. 

#### Marker Structure
<table>
    <tr>
        <th>Name</th>
        <th>Type</th>
        <th>Description</th>
    </tr>
    <tr>
        <td>MarkerSequenceID</td>
        <td>uint64</td>
        <td>The marker sequence identifier of the marker.</td>
    </tr>
    <tr>
        <td>MarkerIndex</td>
        <td>uint64</td>
        <td>The index of marker in the marker sequence.</td>
    </tr>
</table>

#### Create Markers
A new marker is created when:
1. the default conditions are met:
    * **every x messsages**;
    + **every t seconds**;
    + a mix of the first two!
        + Upperbound given by the messages
        + Lower temporal bound given by the time
    + every x messages that reference (directly or indirectly) the previous marker
        + Lower bound given by rank (e.g., how far you are in terms of steps) -> >= 10 or something
        + Upper bound given by the amount of messages referencing the previous one -> ~ 200 msgs
2. A new marker sequence is created. 
> to be confirmed here.
 
A new marker is selected from the tips set randomly, and selected from the weak tips set if there's no strong tip. A new pair of $[MSI, MI]$ is assigned to the new marker. 
> to be confirmed here.

The $MSI$ is set following rules:
* Create a new MSI if it is the first marker of the new marker sequence.
* Inherit the MSI from parents if it references the latest marker of the sequence and meets the requirement to set up a new marker.

The $MI$ is set to $MI = 1+ max(referencedMI)$, which complies the rule:
+ Marker indexes ($MIs$) are monotonically increasing such that $\forall x \in fc(y)$ => $MI_x > MI_y$, where $fc(y)$ is the future cone of $y$ and $x$ is any message in that future cone.

### Past Markers and Future Markers
Each message keeps the marker information in two lists:
* past markers (PMs)
* future markers (FMs)

PMs and FMs are used to determine whether a message is in the past cone of the other, and FMs also help us efficiently estimate the approval weight of a message.

The figure below shows an example of applying markers to the Tangle with 1 marker sequence only:
![](https://i.imgur.com/RluZWCJ.png)

The yellow messages are markers with identifiers: $[0,1]$ and $[0,2]$. Both markers are in $MS$ $0$ with $MI$ $1$ and $2$ respectively.

#### StructureDetails Structure
StructureDetails is a structure that will be in message metadata containing marker information.

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
        <td colspan="2">
            <details open="true">
                <summary>Markers</summary>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>markers</td>
                        <td>map[SequenceID]Index</td>
                        <td>A list of PMs from different marker sequences.</td>
                    </tr>
                </table>
            </details>
        </td>
    </tr>
    <tr>
        <td>FutureMarkers</td>
        <td colspan="2">
            <details open="true">
                <summary>Markers</summary>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>markers</td>
                        <td>map[SequenceID]Index</td>
                        <td>A list of FMs from different marker sequences.</td>
                    </tr>
                </table>
            </details>
        </td>
    </tr>
</table>

##### Past Markers
PMs of a marker is the marker itself only.
PMs of general messages is inherited from its **strong** parents, with 2 steps:
1. keep the nearest markers (i.e., the marker with the biggest $MI$ ) of each marker sequence, 
2. remove those that have been referenced by other markers in the same set. 

##### Future Markers
FMs is empty at start and get updated when the new marker directly or indirectly references it. The FMs propagation ends if:
1. the FMs of a message includes a previous marker of the same marker sequence;
2. the message is the first message we saw in the different marker sequence, we update the FMs of that first message only.

### Markers and Approval Weight
To approximate the approval weight of a message, we simply retrieve the approval weight of its FMs. Since the message is in the past cone of its FMs, the approval weight and the finality will be at least the same as its FMs. This will of course be a lower bound (which is the “safe” bound), but if the markers are set frequently enough, it should be a good approximation.

### Marker Management
Messages can have markers from different marker sequences in PMs and FMs, the order and referenced relationship among marker sequences are important when it comes to inheriting the PMs from parents and other usages. Thus, we need to track these marker sequences.

Here's an example of how markers should look in Tangle:

![](https://i.imgur.com/yi4E3Ik.png)

#### Marker Sequences
Whatever reason a marker sequence is created, we assign a new $MSI = 1+max(referenceMarkerSequences)$. To prevent assigning a new $MSI$ when combining same marker sequences again, we build parents-child relation in a map if a new marker sequence is created. 

#### Rank
The rank of marker sequence graph is the number of sequences from the starting point to itself. The marker sequences in the figure above have rank:
<table>
    <tr>
        <th>Color</th>
        <th>Sequence ID</th>
        <th>Rank</th>
    </tr>
    <tr>
        <td>orange</td>
        <td>0</td>
        <td>1</td>
    </tr>
    <tr>
        <td>blue</td>
        <td>1</td>
        <td>1</td>
    </tr>
    <tr>
        <td>yellow</td>
        <td>2</td>
        <td>2</td>
    </tr>
    <tr>
        <td>green</td>
        <td>3</td>
        <td>1</td>
    </tr>
    <tr>
        <td>red</td>
        <td>4</td>
        <td>2</td>
    </tr>
</table>

We check the parent marker sequences of each candidate in order from high to low rank, if the parent sequence is in the candidate list, it will be removed.

An example is **msg 10** in the figure, the PM candidates are $[0,2], [1,1], [2,3]$. $[2,3]$ is the first marker to check, since it has the highest rank. Then we find its parent marker sequences are $0$ and $1$, and perform futher $MI$ checks. And finally the PMs of **msg 10** is $[2,3]$ only, $[0,2], [1,1]$ are removed.

This function returns the markers and marker sequences to inherit for a message.
```
// normalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *Manager) normalizeMarkers(markers *Markers) (normalizedMarkersByRank *MarkersByRank, normalizedSequences SequenceIDs) {
	rankOfSequencesCache := make(map[SequenceID]uint64)

	normalizedMarkersByRank = NewMarkersByRank()
	normalizedSequences = make(SequenceIDs)
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		normalizedSequences[sequenceID] = types.Void
		normalizedMarkersByRank.Add(m.rankOfSequence(sequenceID, rankOfSequencesCache), sequenceID, index)

		return true
	})
	markersToIterate := normalizedMarkersByRank.Clone()

	for i := markersToIterate.HighestRank() + 1; i > normalizedMarkersByRank.LowestRank(); i-- {
		currentRank := i - 1
		markersByRank, rankExists := markersToIterate.Markers(currentRank)
		if !rankExists {
			continue
		}

		if !markersByRank.ForEach(func(sequenceID SequenceID, index Index) bool {
			if currentRank <= normalizedMarkersByRank.LowestRank() {
				return false
			}

			if !m.sequence(sequenceID).Consume(func(sequence *Sequence) {
				sequence.HighestReferencedParentMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
					delete(normalizedSequences, referencedSequenceID)

					rankOfReferencedSequence := m.rankOfSequence(referencedSequenceID, rankOfSequencesCache)
					if index, indexExists := normalizedMarkersByRank.Index(rankOfReferencedSequence, referencedSequenceID); indexExists {
						if referencedIndex >= index {
							normalizedMarkersByRank.Delete(rankOfReferencedSequence, referencedSequenceID)

							if rankOfReferencedSequence > normalizedMarkersByRank.LowestRank() {
								markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
							}
						}

						return true
					}

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
```
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
    * use epoch j-2 mana, if you're in epoch j 
