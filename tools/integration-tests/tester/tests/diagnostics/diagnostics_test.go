package diagnostics

import (
	"context"
	"fmt"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

var (
	messageHeader = []string{
		"ID", "IssuerID", "IssuerPublicKey", "IssuanceTime", "ArrivalTime", "SolidTime",
		"ScheduledTime", "BookedTime", "GradeOfFinality", "GradeOfFinalityTime", "StrongParents", "WeakParents",
		"DislikeParents", "LikeParents", "StrongApprovers", "WeakApprovers", "BranchID", "Scheduled", "Booked",
		"Invalid", "Rank", "IsPastMarker", "PastMarkers", "PMHI", "PMLI", "FutureMarkers", "FMHI", "FMLI",
		"PayloadType", "TransactionID",
	}

	tipsHeader = messageHeader

	branchesHeader = []string{
		"ID", "ConflictSet", "IssuanceTime", "SolidTime", "LazyBooked", "GradeOfFinality",
	}

	utxoDagHeader = []string{
		"ID", "IssuanceTime", "SolidTime", "AccessManaPledgeID",
		"ConsensusManaPledgeID", "Inputs", "Outputs", "Attachments", "BranchID", "Conflicting", "LazyBooked",
		"GradeOfFinality", "GradeOfFinalityTime",
	}

	drngHeader = []string{
		"ID", "IssuerID", "IssuerPublicKey", "IssuanceTime", "ArrivalTime", "SolidTime",
		"ScheduledTime", "BookedTime", "dRNGPayloadType", "InstanceID", "Round",
		"PreviousSignature", "Signature", "DistributedPK",
	}
)

func TestDiagnosticApis(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 1, framework.CreateNetworkConfig{})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

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
	require.Equal(t, messageHeader, records[0], "unexpected message header")

	fmt.Println("run tools/diagnostic/tips")
	tips, err := peers[0].GoShimmerAPI.GetDiagnosticsTips()
	require.NoError(t, err, "error while performing tools/diagnostic/tips api call")
	records, err = tips.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/tips api csv")
	require.Equal(t, tipsHeader, records[0], "unexpected tips header")

	fmt.Println("run tools/diagnostic/branches")
	branches, err := peers[0].GoShimmerAPI.GetDiagnosticsBranches()
	require.NoError(t, err, "error while running tools/diagnostic/branches")
	records, err = branches.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/branches csv")
	require.Equal(t, branchesHeader, records[0], "unexpected branches header")

	fmt.Println("run tools/diagnostic/utxodag")
	dag, err := peers[0].GoShimmerAPI.GetDiagnosticsUtxoDag()
	require.NoError(t, err, "error while running tools/diagnostic/utxodag api call")
	records, err = dag.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/utxodag csv")
	require.Equal(t, utxoDagHeader, records[0], "unexpected utxoDagHeader header")

	fmt.Println("run tools/diagnostic/drng")
	drng, err := peers[0].GoShimmerAPI.GetDiagnosticsDRNG()
	require.NoError(t, err, "error while running tools/diagnostic/drng api call")
	records, err = drng.ReadAll()
	require.NoError(t, err, "error while reading tools/diagnostic/drng csv")
	require.Equal(t, drngHeader, records[0], "unexpected drngHeader header")
}

func TestSendMessageAPI(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 1, framework.CreateNetworkConfig{
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)
	node := n.Peers()[0]
	api := node.GoShimmerAPI

	sent := tests.SendDataMessages(t, n.Peers()[:1], 5)
	var parentMessageIDs []jsonmodels.ParentMessageIDs
	var messageIDS []string
	for pID := range sent {
		messageIDS = append(messageIDS, pID)
	}
	parentMessageIDs = append(parentMessageIDs, jsonmodels.ParentMessageIDs{
		Type:       uint8(tangle.StrongParentType),
		MessageIDs: messageIDS,
	})

	msgID, err := api.SendMessage(&jsonmodels.SendMessageRequest{
		Payload:          payload.NewGenericDataPayload([]byte("test")).Bytes(),
		ParentMessageIDs: parentMessageIDs,
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, err = node.GetMessage(msgID)
		return err == nil
	}, tests.Timeout, tests.Tick)
}
