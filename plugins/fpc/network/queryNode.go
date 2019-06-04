package network

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	pb "github.com/iotaledger/goshimmer/plugins/fpc/network/query"
	"google.golang.org/grpc"
)

const (
	timeout = 500 * time.Millisecond
)

// queryNode is the internal
func queryNode(client pb.FPCQueryClient, req *pb.QueryRequest) []pb.QueryReply_Opinion {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// preparing undefined opinion in case of errors
	// since we should always return a list of opinions
	// even in case of errors
	undefinedOpinions := make([]pb.QueryReply_Opinion, len(req.GetTxHash()))
	for opinion := range undefinedOpinions {
		undefinedOpinions[opinion] = pb.QueryReply_UNDEFINED
	}

	opinions, err := client.GetOpinion(ctx, req)
	if err != nil {
		log.Printf("%v.GetOpinion(_) = _, %v: \n", client, err)
		return undefinedOpinions
	}
	return opinions.GetOpinion()
}

// QueryNode sends a query to a node and returns a list of opinions
func QueryNode(txHash []fpc.ID, nodeID string) []fpc.Opinion {
	peer, ok := knownpeers.INSTANCE.GetPeer(nodeID)
	if !ok {
		// TODO: if !ok decide what to return
	}
	// TODO: change peer.PeeringPort+2000 with actual port
	nodeEndPoint := peer.Address.String() + ":" + strconv.FormatUint(uint64(peer.PeeringPort+2000), 10)

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	// Connect to the node server
	conn, err := grpc.Dial(nodeEndPoint, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	// Setup a new client over the previous connection
	client := pb.NewFPCQueryClient(conn)

	// Converting fpc.ID to string
	input := make([]string, len(txHash))
	for i := range txHash {
		input[i] = string(txHash[i])
	}
	// Prepare query
	query := &pb.QueryRequest{
		TxHash: input,
	}
	// Send query
	opinions := queryNode(client, query)

	// Converting QueryReply_Opinion to Opinion
	output := make([]fpc.Opinion, len(opinions))
	for i := range opinions {
		output[i] = fpc.Opinion(opinions[i])
	}

	return output
}
