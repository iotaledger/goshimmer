package metrics

import (
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
)

// CollectionEvents defines the events fot the metrics package.
type CollectionEvents struct {
	// AnalysisOutboundBytes defines the local analysis outbound network traffic in bytes.
	AnalysisOutboundBytes *events.Event
	// FPCInboundBytes defines the local FPC inbound network traffic in bytes.
	FPCInboundBytes *events.Event
	// FPCOutboundBytes defines the local FPC outbound network traffic in bytes.
	FPCOutboundBytes *events.Event
	// CPUUsage defines the local CPU usage.
	CPUUsage *events.Event
	// MemUsage defines the local GoShimmer memory usage.
	MemUsage *events.Event
	// Synced defines the local sync status event.
	Synced *events.Event
	// ValueTips defines the local value tips count event.
	ValueTips *events.Event
	// MessageTips defines the local message tips count event.
	MessageTips *events.Event
	// QueryReceived defines the local FPC query received event.
	QueryReceived *events.Event
	// QueryReplyError defines the local FPC query finalization event.
	QueryReplyError *events.Event
	// AnalysisFPCFinalized defines the global FPC finalization event.
	AnalysisFPCFinalized *events.Event
}

// QueryReceivedEvent is used to pass information through a QueryReceived event.
type QueryReceivedEvent struct {
	// OpinionCount defines the local FPC number of opinions requested within a received query.
	OpinionCount int
}

// QueryReplyErrorEvent is used to pass information through a QueryReplyError event.
type QueryReplyErrorEvent struct {
	// ID defines the ID on the queried node.
	ID string
	// OpinionCount defines the local FPC number of opinions requested within a failed query.
	OpinionCount int
}

// AnalysisFPCFinalizedEvent is triggered by the analysis-server to
// notify a finalized FPC vote from one node.
type AnalysisFPCFinalizedEvent struct {
	// ConflictID defines the ID of the finalized conflict.
	ConflictID string
	// NodeID defines the ID of the node.
	NodeID string
	// Rounds defines the number of rounds performed to finalize.
	Rounds int
	// Opinions contains the opinion of each round.
	Opinions []opinion.Opinion
	// Outcome defines the outcome of the FPC voting.
	Outcome opinion.Opinion
}

func queryReceivedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *QueryReceivedEvent))(params[0].(*QueryReceivedEvent))
}

func queryReplyErrorEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *QueryReplyErrorEvent))(params[0].(*QueryReplyErrorEvent))
}

func uint64Caller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}

func float64Caller(handler interface{}, params ...interface{}) {
	handler.(func(float64))(params[0].(float64))
}

func boolCaller(handler interface{}, params ...interface{}) {
	handler.(func(bool))(params[0].(bool))
}

func fpcFinalizedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *AnalysisFPCFinalizedEvent))(params[0].(*AnalysisFPCFinalizedEvent))
}
