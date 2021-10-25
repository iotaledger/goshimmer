package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/goshimmer/client"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func checkMessage(clients []*client.GoShimmerAPI, messageID string, level int) (messages []string) {
	messageMetadata0, err := clients[0].GetMessageMetadata(messageID)
	checkErr(err)
	messageMetadata1, err := clients[1].GetMessageMetadata(messageID)
	checkErr(err)

	if messageMetadata0.BranchID == messageMetadata1.BranchID {
		return []string{messageID}
	}

	fmt.Println(strings.Repeat(" ", level) + messageID)

	message0, err := clients[0].GetMessage(messageID)
	checkErr(err)
	for _, parent := range message0.StrongParents {
		messages = append(messages, checkMessage(clients, parent, level+1)...)
	}

	message1, err := clients[1].GetMessage(messageID)
	checkErr(err)
	for _, parent := range message1.StrongParents {
		messages = append(messages, checkMessage(clients, parent, level+1)...)
	}

	return messages
}

func main() {
	startMessage := "3QZLHsmxyGpMrX8GhWqnhasV9FeEe61ZDhu5P7vKv4Sw"

	apis := []string{
		"http://localhost:8080",
		"http://localhost:8070",
	}

	var clients []*client.GoShimmerAPI
	for _, api := range apis {
		clients = append(clients, client.NewGoShimmerAPI(api, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second})))
	}

	fmt.Println(checkMessage(clients, startMessage, 0))
}
