package client

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/tools"
)

// GetDiagnosticsMessages runs full message diagnostics
// Returns CSV with the following fields:
//
//	ID IssuerID IssuerPublicKey IssuanceTime ArrivalTime SolidTime ScheduledTime BookedTime OpinionFormedTime
//	FinalizedTime StrongParents WeakParents StrongApprovers WeakApprovers BranchID InclusionState Scheduled Booked
//	Eligible Invalid Finalized Rank IsPastMarker PastMarkers PMHI PMLI FutureMarkers FMHI FMLI PayloadType TransactionID
//	PayloadOpinionFormed TimestampOpinionFormed MessageOpinionFormed MessageOpinionTriggered TimestampOpinion
//	TimestampLoK
func (api *GoShimmerAPI) GetDiagnosticsMessages() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticMessages)
}

// GetDiagnosticsFirstWeakMessageReferences runs diagnostics over weak references only.
// Returns CSV with the following fields:
//
//	ID IssuerID IssuerPublicKey IssuanceTime ArrivalTime SolidTime ScheduledTime BookedTime OpinionFormedTime
//	FinalizedTime StrongParents WeakParents StrongApprovers WeakApprovers BranchID InclusionState Scheduled Booked
//	Eligible Invalid Finalized Rank IsPastMarker PastMarkers PMHI PMLI FutureMarkers FMHI FMLI PayloadType TransactionID
//	PayloadOpinionFormed TimestampOpinionFormed MessageOpinionFormed MessageOpinionTriggered TimestampOpinion
//	TimestampLoK
func (api *GoShimmerAPI) GetDiagnosticsFirstWeakMessageReferences() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsFirstWeakMessageReferences)
}

// GetDiagnosticsMessagesByRank run diagnostics for messages whose markers are equal or above a certain rank
// Returns CSV with the following fields:
//
//	ID IssuerID IssuerPublicKey IssuanceTime ArrivalTime SolidTime ScheduledTime BookedTime OpinionFormedTime
//	FinalizedTime StrongParents WeakParents StrongApprovers WeakApprovers BranchID InclusionState Scheduled Booked
//	Eligible Invalid Finalized Rank IsPastMarker PastMarkers PMHI PMLI FutureMarkers FMHI FMLI PayloadType TransactionID
//	PayloadOpinionFormed TimestampOpinionFormed MessageOpinionFormed MessageOpinionTriggered TimestampOpinion
//	TimestampLoK
func (api *GoShimmerAPI) GetDiagnosticsMessagesByRank(rank uint64) (*csv.Reader, error) {
	return api.diagnose(fmt.Sprintf("%s?rank=%d", tools.RouteDiagnosticMessages, rank))
}

// GetDiagnosticsUtxoDag runs diagnostics over utxo dag.
// Returns csv with the following fields:
//
//	ID,IssuanceTime,SolidTime,OpinionFormedTime,AccessManaPledgeID,ConsensusManaPledgeID,Inputs,Outputs,Attachments,
//	BranchID,BranchLiked,BranchMonotonicallyLiked,Conflicting,InclusionState,Finalized,LazyBooked,Liked,LoK,FCOB1Time,
//	FCOB2Time
func (api *GoShimmerAPI) GetDiagnosticsUtxoDag() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsUtxoDag)
}

// GetDiagnosticsBranches runs diagnostics over branches.
// Returns csv with the following fields:
//
//	ID,ConflictSet,IssuanceTime,SolidTime,OpinionFormedTime,Liked,MonotonicallyLiked,InclusionState,Finalized,
//	LazyBooked,TransactionLiked
func (api *GoShimmerAPI) GetDiagnosticsBranches() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsBranches)
}

// GetDiagnosticsLazyBookedBranches runs diagnostics over lazy booked branches.
// Returns csv with the following fields:
//
//	ID,ConflictSet,IssuanceTime,SolidTime,OpinionFormedTime,Liked,MonotonicallyLiked,InclusionState,Finalized,
//	LazyBooked,TransactionLiked
func (api *GoShimmerAPI) GetDiagnosticsLazyBookedBranches() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsLazyBookedBranches)
}

// GetDiagnosticsInvalidBranches runs diagnostics over invalid branches.
// Returns csv with the following fields:
//
//	ID,ConflictSet,IssuanceTime,SolidTime,OpinionFormedTime,Liked,MonotonicallyLiked,InclusionState,Finalized,
//	LazyBooked,TransactionLiked
func (api *GoShimmerAPI) GetDiagnosticsInvalidBranches() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsInvalidBranches)
}

// GetDiagnosticsTips runs diagnostics over tips
// Returns csv with the following fields:
//
//	tipType,ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,OpinionFormedTime,
//	FinalizedTime,StrongParents,WeakParents,StrongApprovers,WeakApprovers,BranchID,InclusionState,Scheduled,Booked,
//	Eligible,Invalid,Finalized,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID,
//	PayloadOpinionFormed,TimestampOpinionFormed,MessageOpinionFormed,MessageOpinionTriggered,TimestampOpinion,TimestampLoK
func (api *GoShimmerAPI) GetDiagnosticsTips() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsTips)
}

// GetDiagnosticsStrongTips runs diagnostics over strong tips
// Returns csv with the following fields:
//
//	tipType,ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,OpinionFormedTime,
//	FinalizedTime,StrongParents,WeakParents,StrongApprovers,WeakApprovers,BranchID,InclusionState,Scheduled,Booked,
//	Eligible,Invalid,Finalized,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID,
//	PayloadOpinionFormed,TimestampOpinionFormed,MessageOpinionFormed,MessageOpinionTriggered,TimestampOpinion,TimestampLoK
func (api *GoShimmerAPI) GetDiagnosticsStrongTips() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsStrongTips)
}

// GetDiagnosticsWeakTips runs diagnostics over weak tips
// Returns csv with the following fields:
//
//	tipType,ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,OpinionFormedTime,
//	FinalizedTime,StrongParents,WeakParents,StrongApprovers,WeakApprovers,BranchID,InclusionState,Scheduled,Booked,
//	Eligible,Invalid,Finalized,Rank,IsPastMarker,PastMarkers,PMHI,PMLI,FutureMarkers,FMHI,FMLI,PayloadType,TransactionID,
//	PayloadOpinionFormed,TimestampOpinionFormed,MessageOpinionFormed,MessageOpinionTriggered,TimestampOpinion,TimestampLoK
func (api *GoShimmerAPI) GetDiagnosticsWeakTips() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsWeakTips)
}

// GetDiagnosticsDRNG runs diagnostics for DRNG
// Returns csv with the following fields:
//
// 	ID,IssuerID,IssuerPublicKey,IssuanceTime,ArrivalTime,SolidTime,ScheduledTime,BookedTime,OpinionFormedTime,
//	dRNGPayloadType,InstanceID,Round,PreviousSignature,Signature,DistributedPK
func (api *GoShimmerAPI) GetDiagnosticsDRNG() (*csv.Reader, error) {
	return api.diagnose(tools.RouteDiagnosticsDRNG)
}

// run an api call on a certain route and return a csv
func (api *GoShimmerAPI) diagnose(route string) (*csv.Reader, error) {
	reader := &csv.Reader{}
	if err := api.do(http.MethodGet, route, nil, reader); err != nil {
		return nil, err
	}
	return reader, nil
}
