package fpc

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// Fpc defines the FPC interface
type Fpc interface {
	SubmitTxsForVoting(txs ...TxOpinion)
	FinalizedTxsChannel() <-chan []TxOpinion // returns a read only channel
}

// Dependencies

// GetKnownPeers defines the signature function
type GetKnownPeers func() (nodeIDs []string)

// QueryNode defines the signature function
type QueryNode func(txs []ID, nodeID string) []Opinion

// Instance defines an FPC object
type Instance struct {
	Fpc
	getKnownPeers       GetKnownPeers
	queryNode           QueryNode
	state               *context
	finalizedTxsChannel chan []TxOpinion // TODO: define what should be the representation of outcome					  // e.g., finalized txs reaching MaxDuration should return -1 or something
}

// New returns a new FPC instance
func New(gkp GetKnownPeers, qn QueryNode, parameters *Parameters) *Instance {
	return &Instance{
		state:               newContext(parameters),
		getKnownPeers:       gkp,
		queryNode:           qn,
		finalizedTxsChannel: make(chan []TxOpinion),
	}
}

// SubmitTxsForVoting adds given txs to the FPC internal state
func (fpc *Instance) SubmitTxsForVoting(txs ...TxOpinion) {
	// TODO: check that txs are not already in state
	fpc.state.pushTxs(txs...)
}

// FinalizedTxsChannel returns the FinalizedTxsChannel
func (fpc *Instance) FinalizedTxsChannel() <-chan []TxOpinion {
	return fpc.finalizedTxsChannel
}

// Tick updates fpc state with the new random
// and starts a new round
func (fpc *Instance) Tick(index uint64, random float64) {
	fpc.state.tick = newTick(index, random)
	go func() { fpc.finalizedTxsChannel <- fpc.round() }()
}

// GetInterimOpinion returns the current opinions
// of the given txs
func (fpc *Instance) GetInterimOpinion(txs ...ID) []Opinion {
	result := make([]Opinion, len(txs))
	for i, tx := range txs {
		if history, ok := fpc.state.opinionHistory.Load(tx); ok {
			lastOpinion, _ := getLastOpinion(history)
			result[i] = lastOpinion
		}
	}
	return result
}

// Hash is the unique identifier of the querried object (e.g. a transaction Hash)
type ID string

// Opinion is a enum
type Opinion int

const (
	// Dislike defines a negative opinion
	Dislike = iota // 0
	// Like defines a negative opinion
	Like // 1
	// Undefined defines an undefined opinion
	Undefined // 2
)

// Opinions is a list of Opinion
type Opinions []Opinion

// TxOpinion defines the current opinion of a tx
// TxHash is the transaction hash
// Opinion is the current opinion
type TxOpinion struct {
	TxHash  ID
	Opinion Opinion
}

// etaMap is a mapping of the ID to the EtaResult
type etaMap map[ID]*etaResult

// newEtaMap returns a new EtaMap
func newEtaMap() etaMap {
	return make(map[ID]*etaResult)
}

// EtaResult defines the eta of an FPC round of a tx
// Value is the value of eta
// Count is how many nodes replied to our query
type etaResult struct {
	value float64
	count int
}

type tick struct {
	index uint64
	x     float64
}

func newTick(index uint64, random float64) *tick {
	return &tick{index, random}
}

// Round performs an FPC round
// i: random number
// i: list of opinions
// i: list of tx to vote
// i: fpc param
// o: list of finalized txs (if any)
func (fpc *Instance) round() []TxOpinion {
	// pop new txs from waiting list and put them into the active list
	fpc.state.popTxs()

	fpc.updateOpinion()

	finalized := fpc.getFinalizedTxs()

	// send the query for all the txs
	etas := querySample(fpc.state.getActiveTxs(), fpc.state.parameters.k, fpc.getKnownPeers(), fpc.queryNode)
	for tx, eta := range etas {
		fpc.state.activeTxs[tx] = eta
	}

	return finalized
}

// returns the last opinion
// i: list of opinions stored during FPC rounds of a particular tx
func getLastOpinion(list Opinions) (Opinion, error) {
	if list != nil && len(list) > 0 {
		return list[len(list)-1], nil
	}
	return Undefined, errors.New("opinion is empty")
}

// // adds a new opinion to a list of opinions
// func updateOpinion(newOpinion Opinion, list Opinions) Opinions {
// 	list = append(list, newOpinion)
// 	return list
// }

// loop over all the txs to vote and update the last opinion
// with the new threshold. If any of them reaches finalization,
// we add those to the finalized list.
func (fpc *Instance) updateOpinion() {
	for tx, eta := range fpc.state.activeTxs {
		if history, ok := fpc.state.opinionHistory.Load(tx); ok && eta.value != -1 {
			threshold := 0.
			if len(history) == 1 { // first round, use a, b
				threshold = runif(fpc.state.tick.x, fpc.state.parameters.a, fpc.state.parameters.b)
			} else { // successive round, use beta
				threshold = runif(fpc.state.tick.x, fpc.state.parameters.beta, 1-fpc.state.parameters.beta)
			}
			//fmt.Println("Tx:",tx, "Eta:", eta.Value, ">", threshold)

			newOpinion := Opinion(Dislike)
			if eta.value > threshold {
				newOpinion = Opinion(Like)
			}
			fpc.state.opinionHistory.Store(tx, newOpinion)
			history = append(history, newOpinion)

		}
	}
}

func (fpc *Instance) getFinalizedTxs() []TxOpinion {
	finalized := []TxOpinion{}
	for tx := range fpc.state.activeTxs {
		history, _ := fpc.state.opinionHistory.Load(tx)
		// note, we check isFinal from [1:] since the first opinion is the initial one
		if isFinal(history[1:], fpc.state.parameters.m, fpc.state.parameters.l) {
			lastOpinion, _ := getLastOpinion(history)
			finalized = append(finalized, TxOpinion{tx, lastOpinion})
			// TODO: op.Delete(tx) ?
			delete(fpc.state.activeTxs, tx)
		}
	}
	return finalized
}

// isFinal returns the finalization status given a list of
// opinions (that belong to a particular tx)
func isFinal(o Opinions, m, l int) bool {
	if o == nil {
		return false
	}
	if len(o) < m+l {
		return false
	}
	finalProposal := o[len(o)-l]
	for _, opinion := range o[len(o)-l+1:] {
		if opinion != finalProposal {
			return false
		}
	}
	return true
}

// querySample sends query to randomly selected nodes
func querySample(txs []ID, k int, nodes []string, qn QueryNode) etaMap {
	// select k random nodes
	selectedNodes := choose(nodes, k)

	// send k queries
	c := make(chan []Opinion, k) // channel to communicate the reception of all the responses
	for _, node := range selectedNodes {
		go func(nodeID string) {
			received := qn(txs, nodeID)
			c <- received
			//fmt.Println("Asked:", txs, "Received:",  received)
		}(node)
	}

	// wait for all the responses and merge them
	result := []TxOpinion{}
	for i := 0; i < k; i++ {
		votes := <-c
		if len(votes) > 0 {
			for voteIdx, vote := range votes {
				result = append(result, TxOpinion{txs[voteIdx], vote})
			}
		}
	}

	return calculateEtas(result)
}

// process the responses by calclulating etas
// for all the votes
func calculateEtas(votes []TxOpinion) etaMap {
	allEtas := make(map[ID]*etaResult)
	for _, vote := range votes {
		if _, ok := allEtas[vote.TxHash]; !ok {
			allEtas[vote.TxHash] = &etaResult{}
		}
		if vote.Opinion != Undefined {
			allEtas[vote.TxHash].value += float64(vote.Opinion)
			allEtas[vote.TxHash].count++
		}

	}
	for tx := range allEtas {
		allEtas[tx].value /= float64(allEtas[tx].count)
	}

	return allEtas
}

func choose(list []string, k int) []string {
	chosen := make([]string, k) // slice containing the list of randomly selected nodes
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < k; i++ {
		chosen[i] = list[rand.Intn(len(list))]
	}
	return chosen
}

// runif returns a random uniform threshold bewteen
// a lower bound and an upper bound
func runif(rand, thresholdL, thresholdU float64) float64 {
	return thresholdL + rand*(thresholdU-thresholdL)
}

// ---------------- utility functions -------------------

func (em etaMap) String() string {
	result := ""
	for k, v := range em {
		result += fmt.Sprintf("tx: %v %v, ", k, v)
	}
	return result
}

func (er etaResult) String() string {
	return fmt.Sprintf("value: %v count: %v", er.value, er.count)
}
