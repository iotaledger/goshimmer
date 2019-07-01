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
	// TIMEOUT is the connection timeout
	TIMEOUT = 500 * time.Millisecond
)

// queryNode is the internal
func queryNode(txHash []fpc.ID, client pb.FPCQueryClient) (output fpc.Opinions) {
	ctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()

	// Converting fpc.ID to string
	input := make([]string, len(txHash))
	for i := range txHash {
		input[i] = string(txHash[i])
	}
	// Prepare query
	query := &pb.QueryRequest{
		TxHash: input,
	}

	output = make(fpc.Opinions, len(txHash))

	opinions, err := client.GetOpinion(ctx, query)
	if err != nil {
		log.Printf("%v.GetOpinion(_) = _, %v: \n", client, err)
		return output
	}

	// Converting QueryReply_Opinion to Opinion
	for i, opinion := range opinions.GetOpinion() {
		output[i] = opinion
	}

	return output
}

// QueryNode sends a query to a node and returns a list of opinions
func QueryNode(txHash []fpc.ID, nodeID string) (opinions fpc.Opinions) {
	peer, _ := knownpeers.INSTANCE.GetPeer(nodeID)

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

	// Send query
	opinions = queryNode(txHash, client)

	return opinions
}
