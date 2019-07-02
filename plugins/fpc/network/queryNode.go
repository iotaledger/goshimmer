package network

import (
	"context"
	"log"
	"strconv"
	"time"
	"unsafe"

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
func queryNode(txHash []fpc.ID, client pb.FPCQueryClient) (output []fpc.Opinion) {
	ctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()

	// Prepare query
	query := &pb.QueryRequest{
		TxHash: *(*[]string)(unsafe.Pointer(&txHash)),
	}

	opinions, err := client.GetOpinion(ctx, query)
	if err != nil {
		log.Printf("%v.GetOpinion(_) = _, %v: \n", client, err)
		return output
	}

	// Converting QueryReply_Opinion to Opinion
	output = *(*[]fpc.Opinion)(unsafe.Pointer(&opinions.Opinion))

	return output
}

// QueryNode sends a query to a node and returns a list of opinions
func QueryNode(txHash []fpc.ID, nodeID string) (opinions []fpc.Opinion) {
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
