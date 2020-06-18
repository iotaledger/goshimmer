package metrics

import (
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/hive.go/events"
)

// CollectionEvents defines the events fot the metrics package
type CollectionEvents struct {
	AnalysisOutboundBytes *events.Event
	FPCInboundBytes       *events.Event
	FPCOutboundBytes      *events.Event
	CPUUsage              *events.Event
	MemUsage              *events.Event
	DBSize                *events.Event
	Synced                *events.Event
	ValueTips             *events.Event
	MessageTips           *events.Event
	QueryReceived         *events.Event
	QueryReplyError       *events.Event
	AnalysisFPCFinalized  *events.Event
}

// QueryReceivedEvent is used to pass information through a QueryReceived event.
type QueryReceivedEvent struct {
	OpinionCount int
}

// QueryReplyErrorEvent is used to pass information through a QueryReplyError event.
type QueryReplyErrorEvent struct {
	ID           string
	OpinionCount int
}

// AnalysisFPCFinalizedEvent is triggered by the analysis-server to
// notify a finalized FPC vote from one node.
type AnalysisFPCFinalizedEvent struct {
	ConflictID string
	NodeID     string
	Rounds     int
	Opinions   []vote.Opinion
	Status     vote.Opinion
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
