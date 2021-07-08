package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/client"
)

// only messages issued in the last timeWindow mins are taken into analysis
var timeWindow = -10 * time.Minute

type schedulingInfo struct {
	avgDelay      int64
	scheduledMsgs int
}

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

	clients[0] = client.NewGoShimmerAPI(masterAPIURL, client.WithHTTPClient(http.Client{Timeout: 180 * time.Second}))
	clients[1] = client.NewGoShimmerAPI(replicaAPIURL, client.WithHTTPClient(http.Client{Timeout: 180 * time.Second}))

	// ignore messages that are issued 10 more mins before now
	endTime := time.Now()

	masterDelayMap := analyzeSchedulingDelay(clients[0], endTime)
	replicaDelayMap := analyzeSchedulingDelay(clients[1], endTime)

	fmt.Println("The average scheduling delay of different issuers on different nodes:")
	fmt.Printf("%-20s %-20s %-15s %-20s %-15s\n\n", "NodeID", "masterNode", "sent msgs", "replicaNode", "sent msgs")
	for nodeID, delay := range masterDelayMap {
		padded := fmt.Sprintf("%-20s %-20v %-15d %-20v %-15d", nodeID, time.Duration(delay.avgDelay)*time.Nanosecond, delay.scheduledMsgs,
			time.Duration(replicaDelayMap[nodeID].avgDelay)*time.Nanosecond, replicaDelayMap[nodeID].scheduledMsgs)
		fmt.Println(padded)
	}
}

func analyzeSchedulingDelay(goshimmerAPI *client.GoShimmerAPI, endTime time.Time) map[string]schedulingInfo {
	csvRes, err := goshimmerAPI.GetDiagnosticsMessages()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	scheduleDelays := calculateSchedulingDelay(csvRes, endTime)

	// the average of delay per node
	avgScheduleDelay := make(map[string]schedulingInfo)
	for nodeID, delays := range scheduleDelays {
		var sum int64 = 0
		for _, d := range delays {
			sum += d.Nanoseconds()
		}
		avgScheduleDelay[nodeID] = schedulingInfo{
			avgDelay:      sum / int64(len(delays)),
			scheduledMsgs: len(delays),
		}
	}

	return avgScheduleDelay
}

func calculateSchedulingDelay(response *csv.Reader, endTime time.Time) map[string][]time.Duration {
	startTime := endTime.Add(timeWindow)
	nodeDelayMap := make(map[string][]time.Duration)
	messageInfos, _ := response.ReadAll()

	for _, msg := range messageInfos {
		arrivalTime := timestampFromString(msg[4])
		// ignore data that is issued before collectTime
		if arrivalTime.Before(startTime) || arrivalTime.After(endTime) {
			continue
		}

		scheduledTime := timestampFromString(msg[6])
		// ignore if the message is not yet scheduled
		if scheduledTime.Before(startTime) || scheduledTime.After(endTime) {
			continue
		}

		issuer := msg[1]
		nodeDelayMap[issuer] = append(nodeDelayMap[issuer], scheduledTime.Sub(arrivalTime))
	}
	return nodeDelayMap
}

func timestampFromString(timeString string) time.Time {
	timeInt64, _ := strconv.ParseInt(timeString, 10, 64)
	return time.Unix(0, timeInt64)
}
