package zeromq

import (
	"context"
	"strconv"
	"strings"

	zmq "github.com/go-zeromq/zmq4"
)

// Simple zmq publisher abstraction
type Publisher struct {
	socket zmq.Socket
}

// Create a new publisher.
func NewPublisher() (*Publisher, error) {

	socket := zmq.NewPub(context.Background())

	return &Publisher{
		socket: socket,
	}, nil
}

// Start the publisher on the given port.
func (pub *Publisher) Start(port int) error {

	return pub.socket.Listen("tcp://*:" + strconv.Itoa(port))
}

// Stop the publisher.
func (pub *Publisher) Shutdown() error {

	return pub.socket.Close()
}

// Publish a new list of messages.
func (pub *Publisher) Send(messages []string) error {
	if len(messages) == 0 || len(messages[0]) == 0 {
		panic("zmq: invalid messages")
	}

	data := strings.Join(messages, " ")
	msg := zmq.NewMsgString(data)
	return pub.socket.Send(msg)
}
