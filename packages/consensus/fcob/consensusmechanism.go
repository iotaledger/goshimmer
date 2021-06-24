package fcob

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timedexecutor"
	"github.com/iotaledger/hive.go/timedqueue"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
)

var (
	// LikedThreshold is the first time threshold of FCoB.
	LikedThreshold = 2 * time.Second

	// LocallyFinalizedThreshold is the second time threshold of FCoB.
	LocallyFinalizedThreshold = 4 * time.Second
)

// region ConsensusMechanism ///////////////////////////////////////////////////////////////////////////////////////////

// ConsensusMechanism represents the FCoB consensus that can be used as a ConsensusMechanism in the Tangle.
type ConsensusMechanism struct {
	Events *ConsensusMechanismEvents

	tangle                   *tangle.Tangle
	Storage                  *Storage
	likedThresholdExecutor   *timedexecutor.TimedExecutor
	locallyFinalizedExecutor *timedexecutor.TimedExecutor
}

// NewConsensusMechanism is the constructor for the FCoB consensus mechanism.
func NewConsensusMechanism() *ConsensusMechanism {
	return &ConsensusMechanism{
		Events: &ConsensusMechanismEvents{
			Error: events.NewEvent(events.ErrorCaller),
			Vote:  events.NewEvent(voteEventHandler),
		},
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
	}
}

// Init initializes the ConsensusMechanism by making the Tangle object available that is using it.
func (f *ConsensusMechanism) Init(tangle *tangle.Tangle) {
	f.tangle = tangle
	f.Storage = NewStorage(tangle.Options.Store, tangle.Options.CacheTimeProvider)
}

// Setup sets up the behavior of the ConsensusMechanism by making it attach to the relevant events in the Tangle.
func (f *ConsensusMechanism) Setup() {
	f.tangle.LedgerState.BranchDAG.Events.BranchConfirmed.Attach(events.NewClosure(func(branchDAGEvent *ledgerstate.BranchDAGEvent) {
		defer branchDAGEvent.Release()
		f.SetTransactionLiked(branchDAGEvent.Branch.ID().TransactionID(), true)
	}))
	f.tangle.LedgerState.BranchDAG.Events.BranchRejected.Attach(events.NewClosure(func(branchDAGEvent *ledgerstate.BranchDAGEvent) {
		defer branchDAGEvent.Release()
		f.SetTransactionLiked(branchDAGEvent.Branch.ID().TransactionID(), false)
	}))

	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.Evaluate))
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.EvaluateTimestamp))
}

// TransactionLiked returns a boolean value indicating whether the given Transaction is liked.
func (f *ConsensusMechanism) TransactionLiked(transactionID ledgerstate.TransactionID) (liked bool) {
	f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		f.tangle.LedgerState.BranchDAG.Branch(transactionMetadata.BranchID()).Consume(func(branch ledgerstate.Branch) {
			if !branch.MonotonicallyLiked() {
				return
			}

			f.Storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
				liked = opinion.OpinionEssence.liked
			})
		})
	})

	return
}

// SetTransactionLiked sets the transaction like status.
func (f *ConsensusMechanism) SetTransactionLiked(transactionID ledgerstate.TransactionID, liked bool) (modified bool) {
	f.Storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
		modified = opinion.SetLiked(liked)
		opinion.SetLevelOfKnowledge(Three)
	})

	return modified
}

// Shutdown shuts down the ConsensusMechanism and persists its state.
func (f *ConsensusMechanism) Shutdown() {
	f.likedThresholdExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.locallyFinalizedExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.Storage.Shutdown()
}

// Evaluate evaluates the opinion of the given messageID.
func (f *ConsensusMechanism) Evaluate(messageID tangle.MessageID) {
	f.Storage.StoreMessageMetadata(NewMessageMetadata(messageID))
	if f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		f.onTransactionBooked(transactionID, messageID)
	}) {
		return
	}

	// likes by default all non-value-transaction messages
	f.onPayloadOpinionFormed(messageID, true)
}

// EvaluateTimestamp evaluates the honesty of the timestamp of the given Message.
func (f *ConsensusMechanism) EvaluateTimestamp(messageID tangle.MessageID) {
	f.Storage.StoreMessageMetadata(NewMessageMetadata(messageID))
	f.Storage.StoreTimestampOpinion(&TimestampOpinion{
		MessageID: messageID,
		Value:     voter.Like,
		LoK:       Two,
	})

	f.setTimestampOpinionDone(messageID)

	if f.messageDone(messageID) {
		f.tangle.Utils.WalkMessageID(f.createMessageOpinion, tangle.MessageIDs{messageID}, true)
	}
}

// ProcessVote allows an external voter to hand in the results of the voting process.
func (f *ConsensusMechanism) ProcessVote(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := ledgerstate.TransactionIDFromBase58(ev.ID)
		if err != nil {
			f.Events.Error.Trigger(err)
			return
		}

		f.Storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLiked(ev.Opinion == voter.Like)
			opinion.SetLevelOfKnowledge(Two)
			// trigger PayloadOpinionFormed event
			messageIDs := f.tangle.Storage.AttachmentMessageIDs(transactionID)
			for _, messageID := range messageIDs {
				f.onPayloadOpinionFormed(messageID, opinion.liked)
			}
		})
	}
}

// TransactionOpinionEssence returns the opinion essence of a given transactionID.
func (f *ConsensusMechanism) TransactionOpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	opinion = f.Storage.OpinionEssence(transactionID)

	return
}

// OpinionsEssence returns a list of OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given conflictSet.
func (f *ConsensusMechanism) OpinionsEssence(targetID ledgerstate.TransactionID, conflictSet ledgerstate.TransactionIDs) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, 0)
	for conflictID := range conflictSet {
		if conflictID == targetID {
			continue
		}
		opinions = append(opinions, f.Storage.OpinionEssence(conflictID))
	}
	return
}

func (f *ConsensusMechanism) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID tangle.MessageID) {
	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	if f.Storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
		// if the opinion has been already set by the opinion provider, re-use it
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormed event
			f.onPayloadOpinionFormed(messageID, opinion.liked)
		}
	}) {
		return
	}

	newOpinion := &Opinion{
		transactionID: transactionID,
	}
	var timestamp time.Time
	f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	// filters both rejected and invalid branch
	branchInclusionState := f.tangle.LedgerState.BranchInclusionState(f.tangle.LedgerState.BranchID(transactionID))
	if branchInclusionState == ledgerstate.Rejected {
		newOpinion.OpinionEssence = OpinionEssence{
			timestamp:        timestamp,
			liked:            false,
			levelOfKnowledge: Two,
		}
		f.Storage.opinionStorage.Store(newOpinion).Release()
		f.onPayloadOpinionFormed(messageID, newOpinion.liked)
		return
	}

	if f.tangle.LedgerState.TransactionConflicting(transactionID) {
		newOpinion.OpinionEssence = deriveOpinion(timestamp, f.OpinionsEssence(transactionID, f.tangle.LedgerState.ConflictSet(transactionID)))

		f.Storage.opinionStorage.Store(newOpinion).Release()

		switch newOpinion.LevelOfKnowledge() {
		case Pending:
			break

		case One:
			// trigger voting for this transactionID
			liked := voter.Dislike
			if newOpinion.liked {
				liked = voter.Like
			}
			f.Events.Vote.Trigger(transactionID.Base58(), liked)
			return

		default:
			f.onPayloadOpinionFormed(messageID, newOpinion.liked)
			return
		}
	}

	newOpinion.OpinionEssence = OpinionEssence{
		timestamp:        timestamp,
		levelOfKnowledge: Pending,
	}
	o, stored := f.Storage.opinionStorage.StoreIfAbsent(newOpinion)
	if stored {
		o.Release()
	}

	// Wait LikedThreshold
	f.likedThresholdExecutor.ExecuteAt(func() {
		runLocallyFinalizedExecutor := true
		if !f.Storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetFCOBTime1(time.Now())

			if f.tangle.LedgerState.TransactionConflicting(transactionID) {
				runLocallyFinalizedExecutor = false
				// if the previous conflicts have been finalized with all dislikes,
				// and no other conflicts arrived within LikedThreshold seconds,
				// start voting with local like
				conflictSet := ConflictSet(f.OpinionsEssence(transactionID, f.tangle.LedgerState.ConflictSet(transactionID)))
				if conflictSet.finalizedAsDisliked(opinion.OpinionEssence) {
					opinion.SetLiked(true)
					opinion.SetLevelOfKnowledge(One)
					// trigger voting for this transactionID
					f.Events.Vote.Trigger(transactionID.Base58(), voter.Like)
					return
				}
				opinion.SetLevelOfKnowledge(One)
				opinion.SetLiked(false)
				// trigger voting for this transactionID
				f.Events.Vote.Trigger(transactionID.Base58(), voter.Dislike)
				return
			}
			opinion.SetLevelOfKnowledge(One)
			opinion.SetLiked(true)
		}) {
			panic(fmt.Sprintf("could not load opinion of transaction %s", transactionID))
		}

		if runLocallyFinalizedExecutor {
			// Wait LocallyFinalizedThreshold
			f.locallyFinalizedExecutor.ExecuteAt(func() {
				if !f.Storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
					opinion.SetFCOBTime2(time.Now())

					opinion.SetLiked(true)
					if f.tangle.LedgerState.TransactionConflicting(transactionID) {
						// trigger voting for this transactionID
						f.Events.Vote.Trigger(transactionID.Base58(), voter.Like)
						return
					}
					opinion.SetLevelOfKnowledge(Two)
					// trigger OpinionPayloadFormed
					messageIDs := f.tangle.Storage.AttachmentMessageIDs(transactionID)
					for _, messageID := range messageIDs {
						f.onPayloadOpinionFormed(messageID, opinion.liked)
					}
				}) {
					panic(fmt.Sprintf("could not load opinion of transaction %s", transactionID))
				}
			}, timestamp.Add(LocallyFinalizedThreshold))
		}
	}, timestamp.Add(LikedThreshold))
}

func (f *ConsensusMechanism) onPayloadOpinionFormed(messageID tangle.MessageID, liked bool) {
	// set BranchLiked if this payload was a conflict and the transaction has not been finalized yet by the approval weight.
	f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			if !transactionMetadata.Finalized() && f.tangle.LedgerState.TransactionConflicting(transactionID) {
				_, err := f.tangle.LedgerState.BranchDAG.SetBranchLiked(f.tangle.LedgerState.BranchID(transactionID), liked)
				if err != nil {
					panic(err)
				}
				if !liked {
					_, err := f.tangle.LedgerState.BranchDAG.SetBranchFinalized(f.tangle.LedgerState.BranchID(transactionID), true)
					if err != nil {
						panic(err)
					}
				}
			}
		})
	})

	f.setPayloadOpinionDone(messageID)

	if f.messageDone(messageID) {
		f.tangle.Utils.WalkMessageID(f.createMessageOpinion, tangle.MessageIDs{messageID}, true)
	}
}

func (f *ConsensusMechanism) createMessageOpinion(messageID tangle.MessageID, walker *walker.Walker) {
	if !f.parentsDone(messageID) {
		return
	}

	if !f.setMessageOpinionFormed(messageID) {
		return
	}

	f.tangle.ConsensusManager.Events.MessageOpinionFormed.Trigger(messageID)

	f.setMessageOpinionTriggered(messageID)

	f.tangle.Storage.Approvers(messageID).Consume(func(approver *tangle.Approver) {
		if f.messageDone(approver.ApproverMessageID()) {
			walker.Push(approver.ApproverMessageID())
		}
	})
}

func (f *ConsensusMechanism) setEligibility(messageID tangle.MessageID) {
	f.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		f.Storage.TimestampOpinion(messageID).Consume(func(timestampOpinion *TimestampOpinion) {
			messageMetadata.SetEligible(
				timestampOpinion != nil && timestampOpinion.Value == opinion.Like && timestampOpinion.LoK > One && f.parentsEligibility(messageID),
			)
		})
	})
}

// OpinionFormedTime returns the time when the opinion for the given message was formed.
func (f *ConsensusMechanism) OpinionFormedTime(messageID tangle.MessageID) (t time.Time) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		t = messageMetadata.OpinionFormedTime()
	})
	return
}

// parentsEligibility checks if the parents are eligible.
func (f *ConsensusMechanism) parentsEligibility(messageID tangle.MessageID) (eligible bool) {
	f.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent tangle.Parent) {
			if eligible = eligible && f.tangle.ConsensusManager.MessageEligible(parent.ID); !eligible {
				return
			}
		})
	})
	return
}

// setPayloadOpinionDone set the payload opinion as formed.
func (f *ConsensusMechanism) setPayloadOpinionDone(messageID tangle.MessageID) (modified bool) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		modified = messageMetadata.SetPayloadOpinionFormed(true)
	})
	return
}

// setTimestampOpinionDone set the timestamp opinion as formed.
func (f *ConsensusMechanism) setTimestampOpinionDone(messageID tangle.MessageID) (modified bool) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		modified = messageMetadata.SetTimestampOpinionFormed(true)
	})
	return
}

// setMessageOpinionFormed set the message opinion as formed.
func (f *ConsensusMechanism) setMessageOpinionFormed(messageID tangle.MessageID) (modified bool) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		modified = messageMetadata.SetMessageOpinionFormed(true)
	})
	return
}

// messageDone checks if the both timestamp opinion and payload opinion are formed.
func (f *ConsensusMechanism) messageDone(messageID tangle.MessageID) (done bool) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		done = messageMetadata.TimestampOpinionFormed() && messageMetadata.PayloadOpinionFormed()
	})
	return
}

// setMessageOpinionTriggered set the message opinion as triggered.
func (f *ConsensusMechanism) setMessageOpinionTriggered(messageID tangle.MessageID) (modified bool) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		modified = messageMetadata.SetMessageOpinionTriggered(true)
	})
	return
}

// messageOpinionTriggered checks if the message opinion has been triggered.
func (f *ConsensusMechanism) messageOpinionTriggered(messageID tangle.MessageID) (messageOpinionTriggered bool) {
	f.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		messageOpinionTriggered = messageMetadata.MessageOpinionTriggered()
	})
	return
}

// parentsDone checks if all of the parents' opinion are formed.
func (f *ConsensusMechanism) parentsDone(messageID tangle.MessageID) (done bool) {
	f.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		done = true
		message.ForEachParent(func(parent tangle.Parent) {
			if !done {
				return
			}
			done = f.messageOpinionTriggered(parent.ID) && done
		})
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusMechanismEvents /////////////////////////////////////////////////////////////////////////////////////

// ConsensusMechanismEvents represents events triggered by the ConsensusMechanism.
type ConsensusMechanismEvents struct {
	// Error gets called when FCOB faces an error.
	Error *events.Event

	// Vote gets called when FCOB needs to vote.
	Vote *events.Event
}

func voteEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(id string, initOpn voter.Opinion))(params[0].(string), params[1].(voter.Opinion))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region utility functions ////////////////////////////////////////////////////////////////////////////////////////////

// deriveOpinion returns the initial opinion based on the given targetTime and conflictSet.
func deriveOpinion(targetTime time.Time, conflictSet ConflictSet) (opinion OpinionEssence) {
	if conflictSet.hasDecidedLike() {
		opinion = OpinionEssence{
			timestamp:        targetTime,
			liked:            false,
			levelOfKnowledge: Two,
		}
		return
	}

	anchor := conflictSet.anchor()
	if (anchor == OpinionEssence{}) {
		opinion = OpinionEssence{
			timestamp:        targetTime,
			levelOfKnowledge: Pending,
		}
		return
	}

	opinion = OpinionEssence{
		timestamp:        targetTime,
		liked:            false,
		levelOfKnowledge: One,
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
