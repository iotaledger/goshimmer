package fpc

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/hive.go/events"
)

var (
	ErrVoteAlreadyOngoing       = errors.New("a vote is already ongoing for the given ID")
	ErrNoOpinionGiversAvailable = errors.New("can't perform round as no opinion givers are available")
)

// New creates a new FPC instance.
func New(opinionGiverFunc vote.OpinionGiverFunc, paras ...*Parameters) *FPC {
	f := &FPC{
		opinionGiverFunc: opinionGiverFunc,
		paras:            DefaultParameters(),
		opinionGiverRng:  rand.New(rand.NewSource(time.Now().UnixNano())),
		ctxs:             make(map[string]*vote.Context),
		queue:            list.New(),
		queueSet:         make(map[string]struct{}),
		events: vote.Events{
			Finalized:     events.NewEvent(vote.OpinionCaller),
			Failed:        events.NewEvent(vote.OpinionCaller),
			RoundExecuted: events.NewEvent(vote.RoundStatsCaller),
			Error:         events.NewEvent(events.ErrorCaller),
		},
	}
	if len(paras) > 0 {
		f.paras = paras[0]
	}
	return f
}

// FPC is a DRNGRoundBasedVoter which uses the Opinion of other entities
// in order to finalize an Opinion.
type FPC struct {
	events           vote.Events
	opinionGiverFunc vote.OpinionGiverFunc
	// the lifo queue of newly enqueued items to vote on.
	queue *list.List
	// contains a set of currently queued items.
	queueSet map[string]struct{}
	queueMu  sync.Mutex
	// contains the set of current vote contexts.
	ctxs   map[string]*vote.Context
	ctxsMu sync.RWMutex
	// parameters to use within FPC.
	paras *Parameters
	// indicates whether the last round was performed successfully.
	lastRoundCompletedSuccessfully bool
	// used to randomly select opinion givers.
	opinionGiverRng *rand.Rand
}

func (f *FPC) Vote(id string, initOpn vote.Opinion) error {
	f.queueMu.Lock()
	defer f.queueMu.Unlock()
	f.ctxsMu.RLock()
	defer f.ctxsMu.RUnlock()
	if _, alreadyQueued := f.queueSet[id]; alreadyQueued {
		return fmt.Errorf("%w: %s", ErrVoteAlreadyOngoing, id)
	}
	if _, alreadyOngoing := f.ctxs[id]; alreadyOngoing {
		return fmt.Errorf("%w: %s", ErrVoteAlreadyOngoing, id)
	}
	f.queue.PushBack(vote.NewContext(id, initOpn))
	f.queueSet[id] = struct{}{}
	return nil
}

func (f *FPC) IntermediateOpinion(id string) (vote.Opinion, error) {
	f.ctxsMu.RLock()
	defer f.ctxsMu.RUnlock()
	voteCtx, has := f.ctxs[id]
	if !has {
		return vote.Unknown, fmt.Errorf("%w: %s", vote.ErrVotingNotFound, id)
	}
	return voteCtx.LastOpinion(), nil
}

func (f *FPC) Events() vote.Events {
	return f.events
}

// Round enqueues new items, sets opinions on active vote contexts, finalizes them and then
// queries for opinions.
func (f *FPC) Round(rand float64) error {
	start := time.Now()
	// enqueue new voting contexts
	f.enqueue()
	// we can only form opinions when the last round was actually executed successfully
	if f.lastRoundCompletedSuccessfully {
		// form opinions by using the random number supplied for this new round
		f.formOpinions(rand)
		// clean opinions on vote contexts where an opinion was reached in FinalizationThreshold
		// number of rounds and clear those who failed to be finalized in MaxRoundsPerVoteContext.
		f.finalizeOpinions()
	}
	// query for opinions on the current vote contexts
	queriedOpinions, err := f.queryOpinions()
	if err == nil {
		f.lastRoundCompletedSuccessfully = true
		// execute a round executed event
		roundStats := &vote.RoundStats{
			Duration:           time.Since(start),
			RandUsed:           rand,
			ActiveVoteContexts: f.ctxs,
			QueriedOpinions:    queriedOpinions,
		}
		// TODO: add possibility to check whether an event handler is registered
		// in order to prevent the collection of the round stats data if not needed
		f.events.RoundExecuted.Trigger(roundStats)
	}
	return err
}

// enqueues items for voting
func (f *FPC) enqueue() {
	f.queueMu.Lock()
	defer f.queueMu.Unlock()
	f.ctxsMu.Lock()
	defer f.ctxsMu.Unlock()
	for ele := f.queue.Front(); ele != nil; ele = f.queue.Front() {
		voteCtx := ele.Value.(*vote.Context)
		f.ctxs[voteCtx.ID] = voteCtx
		f.queue.Remove(ele)
		delete(f.queueSet, voteCtx.ID)
	}
}

// formOpinions updates the opinion for ongoing vote contexts by comparing their liked percentage
// against the threshold appropriate for their given rounds.
func (f *FPC) formOpinions(rand float64) {
	f.ctxsMu.RLock()
	defer f.ctxsMu.RUnlock()
	for _, voteCtx := range f.ctxs {
		// when the vote context is new there's no opinion to form
		if voteCtx.IsNew() {
			continue
		}

		lowerThreshold := f.paras.SubsequentRoundsLowerBoundThreshold
		upperThreshold := f.paras.SubsequentRoundsUpperBoundThreshold

		if voteCtx.HadFirstRound() {
			lowerThreshold = f.paras.FirstRoundLowerBoundThreshold
			upperThreshold = f.paras.FirstRoundUpperBoundThreshold
		}

		if voteCtx.Liked >= RandUniformThreshold(rand, lowerThreshold, upperThreshold) {
			voteCtx.AddOpinion(vote.Like)
			continue
		}
		voteCtx.AddOpinion(vote.Dislike)
	}
}

// emits a Voted event for every finalized vote context (or Failed event if failed) and then removes it from FPC.
func (f *FPC) finalizeOpinions() {
	f.ctxsMu.Lock()
	defer f.ctxsMu.Unlock()
	for id, voteCtx := range f.ctxs {
		if voteCtx.IsFinalized(f.paras.CoolingOffPeriod, f.paras.FinalizationThreshold) {
			f.events.Finalized.Trigger(&vote.OpinionEvent{ID: id, Opinion: voteCtx.LastOpinion(), Ctx: *voteCtx})
			delete(f.ctxs, id)
			continue
		}
		if voteCtx.Rounds >= f.paras.MaxRoundsPerVoteContext {
			f.events.Failed.Trigger(&vote.OpinionEvent{ID: id, Opinion: voteCtx.LastOpinion(), Ctx: *voteCtx})
			delete(f.ctxs, id)
		}
	}
}

// queries the opinions of QuerySampleSize amount of OpinionGivers.
func (f *FPC) queryOpinions() ([]vote.QueriedOpinions, error) {
	ids := f.voteContextIDs()

	// nothing to vote on
	if len(ids) == 0 {
		return nil, nil
	}

	opinionGivers, err := f.opinionGiverFunc()
	if err != nil {
		return nil, err
	}

	// nobody to query
	if len(opinionGivers) == 0 {
		return nil, ErrNoOpinionGiversAvailable
	}

	// select a random subset of opinion givers to query.
	// if the same opinion giver is selected multiple times, we query it only once
	// but use its opinion N selected times.
	opinionGiversToQuery := map[vote.OpinionGiver]int{}
	for i := 0; i < f.paras.QuerySampleSize; i++ {
		selected := opinionGivers[f.opinionGiverRng.Intn(len(opinionGivers))]
		opinionGiversToQuery[selected]++
	}

	// votes per id
	var voteMapMu sync.Mutex
	voteMap := map[string]vote.Opinions{}

	// holds queried opinions
	allQueriedOpinions := []vote.QueriedOpinions{}

	// send queries
	var wg sync.WaitGroup
	for opinionGiverToQuery, selectedCount := range opinionGiversToQuery {
		wg.Add(1)
		go func(opinionGiverToQuery vote.OpinionGiver, selectedCount int) {
			defer wg.Done()

			queryCtx, cancel := context.WithTimeout(context.Background(), f.paras.QueryTimeout)
			defer cancel()

			// query
			opinions, err := opinionGiverToQuery.Query(queryCtx, ids)
			if err != nil || len(opinions) != len(ids) {
				// ignore opinions
				return
			}

			queriedOpinions := vote.QueriedOpinions{
				OpinionGiverID: opinionGiverToQuery.ID(),
				Opinions:       make(map[string]vote.Opinion),
				TimesCounted:   selectedCount,
			}

			// add opinions to vote map
			voteMapMu.Lock()
			defer voteMapMu.Unlock()
			for i, id := range ids {
				votes, has := voteMap[id]
				if !has {
					votes = vote.Opinions{}
				}
				// reuse the opinion N times selected.
				// note this is always at least 1.
				for j := 0; j < selectedCount; j++ {
					votes = append(votes, opinions[i])
				}
				queriedOpinions.Opinions[id] = opinions[i]
				voteMap[id] = votes
			}
			allQueriedOpinions = append(allQueriedOpinions, queriedOpinions)
		}(opinionGiverToQuery, selectedCount)
	}
	wg.Wait()

	f.ctxsMu.RLock()
	defer f.ctxsMu.RUnlock()
	// compute liked percentage
	for id, votes := range voteMap {
		var likedSum float64
		votedCount := float64(len(votes))

		for _, opinion := range votes {
			switch opinion {
			case vote.Unknown:
				votedCount--
			case vote.Like:
				likedSum++
			}
		}

		// mark a round being done, even though there's no opinion,
		// so this voting context will be cleared eventually
		f.ctxs[id].Rounds++
		if votedCount == 0 {
			continue
		}
		f.ctxs[id].Liked = likedSum / votedCount
	}
	return allQueriedOpinions, nil
}

func (f *FPC) voteContextIDs() []string {
	f.ctxsMu.RLock()
	defer f.ctxsMu.RUnlock()
	var i int
	ids := make([]string, len(f.ctxs))
	for id := range f.ctxs {
		ids[i] = id
		i++
	}
	return ids
}
