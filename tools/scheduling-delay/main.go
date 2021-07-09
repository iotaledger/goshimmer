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
	apiUrls := []string{
		// master node has around 66.3% of a-mana (the master node in the docker-network)
		"http://127.0.0.1:8080",
		// replica node has only 1 a-mana (peer_replicas in the docker-network)
		"http://127.0.0.1:8070",
	}

	clients := createGoShimmerClients(apiUrls)

	endTime := time.Now()
	delayMaps := make(map[string]map[string]schedulingInfo, len(apiUrls))
	for _, client := range clients {
		nodeInfo, err := client.Info()
		if err != nil {
			fmt.Println(client.BaseURL(), "crashed")
			continue
		}
		delayMaps[nodeInfo.IdentityIDShort] = analyzeSchedulingDelay(client, endTime)
	}

	printResults(delayMaps)
}

func createGoShimmerClients(apiUrls []string) []*client.GoShimmerAPI {
	clients := make([]*client.GoShimmerAPI, len(apiUrls))
	for i, url := range apiUrls {
		clients[i] = client.NewGoShimmerAPI(url, client.WithHTTPClient(http.Client{Timeout: 180 * time.Second}))
	}
	return clients
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

func printResults(delayMaps map[string]map[string]schedulingInfo) {
	fmt.Printf("The average scheduling delay of different issuers on different nodes:\n\n")

	var nodeOrder []string
	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for nodeID := range delayMaps {
		nodeOrder = append(nodeOrder, nodeID)
		title = fmt.Sprintf("%s %-15s %-15s", title, nodeID, "scheduled msgs")
	}
	fmt.Printf("%s\n\n", title)

	for _, issuer := range nodeOrder {
		row := fmt.Sprintf("%-15s", issuer)
		for _, nodeID := range nodeOrder {
			row = fmt.Sprintf("%s %-15v %-15d", row, time.Duration(delayMaps[nodeID][issuer].avgDelay)*time.Nanosecond,
				delayMaps[nodeID][issuer].scheduledMsgs)
		}
		fmt.Println(row)
	}
}
