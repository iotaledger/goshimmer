package diagnostics

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
)

var (
	messageHeader = []string{
		"ID", "IssuerID", "IssuerPublicKey", "IssuanceTime", "ArrivalTime", "SolidTime",
		"ScheduledTime", "BookedTime", "OpinionFormedTime", "FinalizedTime", "StrongParents", "WeakParents",
		"StrongApprovers", "WeakApprovers", "BranchID", "InclusionState", "Scheduled", "Booked", "Eligible", "Invalid",
		"Finalized", "Rank", "IsPastMarker", "PastMarkers", "PMHI", "PMLI", "FutureMarkers", "FMHI", "FMLI", "PayloadType",
		"TransactionID", "PayloadOpinionFormed", "TimestampOpinionFormed", "MessageOpinionFormed",
		"MessageOpinionTriggered", "TimestampOpinion", "TimestampLoK",
	}

	tipsHeader = append([]string{"tipType"}, messageHeader...)

	branchesHeader = []string{
		"ID", "ConflictSet", "IssuanceTime", "SolidTime", "OpinionFormedTime", "Liked",
		"MonotonicallyLiked", "InclusionState", "Finalized", "LazyBooked", "TransactionLiked",
	}

	utxoDagHeader = []string{
		"ID", "IssuanceTime", "SolidTime", "OpinionFormedTime", "AccessManaPledgeID",
		"ConsensusManaPledgeID", "Inputs", "Outputs", "Attachments", "BranchID", "BranchLiked", "BranchMonotonicallyLiked",
		"Conflicting", "InclusionState", "Finalized", "LazyBooked", "Liked", "LoK", "FCOB1Time", "FCOB2Time",
	}

	drngHeader = []string{
		"ID", "IssuerID", "IssuerPublicKey", "IssuanceTime", "ArrivalTime", "SolidTime",
		"ScheduledTime", "BookedTime", "OpinionFormedTime", "dRNGPayloadType", "InstanceID", "Round",
		"PreviousSignature", "Signature", "DistributedPK",
	}
)

func TestDiagnosticApis(t *testing.T) {
	n, err := f.CreateNetwork("diagnostics_TestAPI", 1, framework.CreateNetworkConfig{Faucet: false, Mana: false})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)
	time.Sleep(10 * time.Second)
	peers := n.Peers()
	for _, p := range peers {
		fmt.Printf("peer id: %s, short id: %s\n", base58.Encode(p.ID().Bytes()), p.ID().String())
	}

	fmt.Println("run /diagnostic/messages")
	api := peers[0].GoShimmerAPI
	fmt.Println("get api")
	resp, err := api.GetDiagnosticsMessages()
	require.NoError(t, err, "error while performing /diagnostic/messages api call")
	records, err := resp.ReadAll()
	require.NoError(t, err, "error while reading  /diagnostic/messages csv")
	require.Equal(t, records[0], messageHeader, "unexpected message header")

	fmt.Println("run tools/diagnostic/messages/firstweakreferences")
	resp, err = peers[0].GoShimmerAPI.GetDiagnosticsFirstWeakMessageReferences()
	require.NoError(t, err, "error while performing tools/diagnostic/messages/firstweakreferences api call")
	records, err = resp.ReadAll()
	require.NoError(t, err, "error while reading  /diagnostic/messages/firstweakreferences csv")
	require.Equal(t, records[0], messageHeader, "unexpected message header")

	fmt.Println("run tools/diagnostic/tips")
	tips, err := peers[0].GoShimmerAPI.GetDiagnosticsTips()
	require.NoError(t, err, "error while performing tools/diagnostic/tips api call")
	records, err = tips.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/tips api csv")
	require.Equal(t, records[0], tipsHeader, "unexpected tips header")

	fmt.Println("run tools/diagnostic/tips/strong")
	strongTips, err := peers[0].GoShimmerAPI.GetDiagnosticsStrongTips()
	require.NoError(t, err, "error while running tools/diagnostic/tips/strong api call")
	records, err = strongTips.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/tips/strong api csv")
	require.Equal(t, records[0], tipsHeader, "unexpected tips header")

	fmt.Println("run tools/diagnostic/tips/weak")
	weakTips, err := peers[0].GoShimmerAPI.GetDiagnosticsWeakTips()
	require.NoError(t, err, "error while running tools/diagnostic/tips/weak api call")
	records, err = weakTips.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/tips/weak api csv")
	require.Equal(t, records[0], tipsHeader, "unexpected tips header")

	fmt.Println("run tools/diagnostic/branches")
	branches, err := peers[0].GoShimmerAPI.GetDiagnosticsBranches()
	require.NoError(t, err, "error while running tools/diagnostic/branches")
	records, err = branches.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/branches csv")
	require.Equal(t, records[0], branchesHeader, "unexpected branches header")

	fmt.Println("run tools/diagnostic/branches/lazybooked")
	lazyBookedBranches, err := peers[0].GoShimmerAPI.GetDiagnosticsLazyBookedBranches()
	require.NoError(t, err, "error while running tools/diagnostic/branches/lazybooked api call")
	records, err = lazyBookedBranches.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/branches/lazybooked csv")
	require.Equal(t, records[0], branchesHeader, "unexpected tips header")

	fmt.Println("run tools/diagnostic/branches/invalid")
	invalidBranches, err := peers[0].GoShimmerAPI.GetDiagnosticsInvalidBranches()
	require.NoError(t, err, "error while running tools/diagnostic/branches/invalid api call")
	records, err = invalidBranches.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/branches/invalid csv")
	require.Equal(t, records[0], branchesHeader, "unexpected tips header")

	fmt.Println("run tools/diagnostic/utxodag")
	dag, err := peers[0].GoShimmerAPI.GetDiagnosticsUtxoDag()
	require.NoError(t, err, "error while running tools/diagnostic/utxodag api call")
	records, err = dag.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/utxodag csv")
	require.Equal(t, records[0], utxoDagHeader, "unexpected utxoDagHeader header")

	fmt.Println("run tools/diagnostic/drng")
	drng, err := peers[0].GoShimmerAPI.GetDiagnosticsDRNG()
	require.NoError(t, err, "error while running tools/diagnostic/drng api call")
	records, err = drng.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/drng csv")
	require.Equal(t, records[0], drngHeader, "unexpected drngHeader header")
}
