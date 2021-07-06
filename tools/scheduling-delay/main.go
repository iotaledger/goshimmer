package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/client"
)

func main() {
	clients := make([]*client.GoShimmerAPI, 2)

	// master node has around 66.3% of a-mana (the master node in the docker-network)
	masterAPIURL := "http://127.0.0.1:8080"
	// replica node has only 1 a-mana (peer_replicas in the docker-network)
	replicaAPIURL := "http://127.0.0.1:8070"

	if masterAPIURL == replicaAPIURL {
		fmt.Println("Please use 2 different nodes to issue a double-spend")
		return
	}

	clients[0] = client.NewGoShimmerAPI(masterAPIURL, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))
	clients[1] = client.NewGoShimmerAPI(replicaAPIURL, client.WithHTTPClient(http.Client{Timeout: 60 * time.Second}))

	// ignore messages that are issued 10 more mins before now
	collectTime := time.Now().Add(-10 * time.Minute)
	masterDelayMap := analyzeSchedulingDelay(clients[0], collectTime)
	replicaDelayMap := analyzeSchedulingDelay(clients[1], collectTime)

	fmt.Println("The average scheduling delay of different issuers on different nodes:")
	fmt.Printf("%-20s %-20s %-20s\n\n", "NodeID", "masterNode", "replicaNode")
	for nodeID, delay := range masterDelayMap {
		padded := fmt.Sprintf("%-20s %-20v %-20v", nodeID, time.Duration(delay)*time.Nanosecond, time.Duration(replicaDelayMap[nodeID])*time.Nanosecond)
		fmt.Println(padded)
	}
}

func analyzeSchedulingDelay(goshimmerAPI *client.GoShimmerAPI, collectTime time.Time) map[string]int64 {
	csvRes, err := goshimmerAPI.GetDiagnosticsMessages()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	scheduleDelays := calculateSchedulingDelay(csvRes, collectTime)

	// the average of delay per node
	avgScheduleDelay := make(map[string]int64)
	for nodeID, delays := range scheduleDelays {
		var sum int64 = 0
		for _, d := range delays {
			sum += d.Nanoseconds()
		}
		avgScheduleDelay[nodeID] = sum / int64(len(delays))
	}

	return avgScheduleDelay
}

func calculateSchedulingDelay(response *csv.Reader, collectTime time.Time) map[string][]time.Duration {
	nodeDelayMap := make(map[string][]time.Duration)
	messageInfos, _ := response.ReadAll()

	for _, msg := range messageInfos {
		issueTime := timestampFromString(msg[3])
		// ignore data that is issued before collectTime
		if issueTime.Before(collectTime) {
			continue
		}

		scheduledTime := timestampFromString(msg[6])
		// ignore if the message is not yet scheduled
		if scheduledTime.Before(collectTime) {
			continue
		}

		issuer := msg[1]
		nodeDelayMap[issuer] = append(nodeDelayMap[issuer], scheduledTime.Sub(issueTime))
	}
	return nodeDelayMap
}

func timestampFromString(timeString string) time.Time {
	timeInt64, _ := strconv.ParseInt(timeString, 10, 64)
	return time.Unix(0, timeInt64)
}
