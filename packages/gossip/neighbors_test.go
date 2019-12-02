package gossip

import (
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/buffstreams"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
)

// TestCallback is a simple server for test purposes. It has a single callback,
// which is to unmarshall some data and log it.
func (t *testController) TestCallback(bts []byte) error {
	msg := &pb.ConnectionRequest{
		// Version:   1,
		// From:      "client",
		// To:        "server",
		// Timestamp: 1,
	}
	err := proto.Unmarshal(bts, msg)
	if t.enableLogging {
		fmt.Println(msg.GetFrom(), msg.GetTo())
	}
	return err
}

type testController struct {
	enableLogging bool
}

func server() {
	tc := &testController{
		enableLogging: true,
	}
	cfg := buffstreams.TCPListenerConfig{
		MaxMessageSize: 2048,
		EnableLogging:  tc.enableLogging,
		Address:        buffstreams.FormatAddress("127.0.0.1", strconv.Itoa(5031)),
		Callback:       tc.TestCallback,
	}
	bm := buffstreams.NewManager()
	err := bm.StartListening(cfg)
	if err != nil {
		log.Print(err)
	} else {
		// Need to block until ctrl+c, but having trouble getting signal trapping to work on OSX...
		time.Sleep(time.Second * 2)
		fmt.Println(bm.GetConnectionIDs())
	}
}

func client() {
	cfg := &buffstreams.TCPConnConfig{
		MaxMessageSize: 2048,
		Address:        buffstreams.FormatAddress("127.0.0.1", strconv.Itoa(5031)),
	}
	msg := &pb.ConnectionRequest{
		Version:   1,
		From:      "client",
		To:        "server",
		Timestamp: 1,
	}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Print(err)
	}
	//count := 0
	cbm := buffstreams.NewManager()
	err = cbm.Dial(cfg)
	if err != nil {
		log.Fatal(err)
	}
	btw, err := cbm.Write(cfg.Address, msgBytes)
	//btw, err := buffstreams.DialTCP(cfg)
	if err != nil {
		log.Fatal(btw, err)
	}
	// currentTime := time.Now()
	// lastTime := currentTime
	// for {
	// 	_, err := btw.Write(msgBytes)
	// 	if err != nil {
	// 		fmt.Println("There was an error")
	// 		fmt.Println(err)
	// 	}
	// 	count = count + 1
	// 	if lastTime.Second() != currentTime.Second() {
	// 		lastTime = currentTime
	// 		fmt.Printf(", %d\n", count)
	// 		count = 0
	// 	}
	// 	currentTime = time.Now()
	// }
}
func TestDummy(t *testing.T) {
	// connections = make(map[string]net.Conn)
	// makeListener()

	// conn, err := net.Dial("tcp", "localhost:8080")
	// if err != nil {
	// 	// handle error
	// }
	// testMessage := new(pb.ConnectionRequest)
	// testMessage.Version = 1
	// testMessage.From = conn.LocalAddr().String()
	// testMessage.To = conn.RemoteAddr().String()
	// testMessage.Timestamp = 1
	// pkt, err := proto.Marshal(testMessage)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// n, err := conn.Write(pkt)
	// if err != nil {
	// 	fmt.Println(n, err)
	// }
	// //status, err := bufio.NewReader(conn).ReadString('\n')
	// time.Sleep(4 * time.Second)

	go server()
	go client()
	time.Sleep(4 * time.Second)
}
