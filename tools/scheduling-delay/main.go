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

type MPSInfo struct {
	mps float64
}
type schedulingInfo struct {
	avgDelay      int64
	scheduledMsgs int
	nodeQLen      int
}

func main() {
	apiUrls := []string{}

	clients := createGoShimmerClients(apiUrls)

	// start spamming
	resp, err := clients[0].ToggleSpammer(true, 574)
	if err != nil {
		panic(err)
	}
	fmt.Println("Bootstrap 1 spamming at 574", resp)

	resp, err = clients[1].ToggleSpammer(true, 154)
	if err != nil {
		panic(err)
	}
	fmt.Println("Bootstrap 2 spamming at 154", resp)

	resp, err = clients[2].ToggleSpammer(true, 274)
	if err != nil {
		panic(err)
	}
	fmt.Println("Faucet spamming at 274", resp)

	resp, err = clients[3].ToggleSpammer(true, 52)
	if err != nil {
		panic(err)
	}
	fmt.Println("Falk 1 spamming at 52", resp)

	resp, err = clients[4].ToggleSpammer(true, 4)
	if err != nil {
		panic(err)
	}
	fmt.Println("Falk 2 spamming at 4", resp)

	fmt.Println(time.Now())

	time.Sleep(11 * time.Minute)

	endTime := time.Now()
	delayMaps := make(map[string]map[string]schedulingInfo, len(apiUrls))
	mpsMaps := make(map[string]map[string]MPSInfo, len(apiUrls))
	for _, client := range clients {
		nodeInfo, err := client.Info()
		if err != nil {
			fmt.Println(client.BaseURL(), "crashed")
			continue
		}
		delayMaps[nodeInfo.IdentityIDShort] = analyzeSchedulingDelay(client, endTime)
		mpsMaps[nodeInfo.IdentityIDShort] = analyzeMPSDistribution(client, endTime)
		// get node queue sizes
		for issuer, qLen := range nodeInfo.Scheduler.NodeQueueSizes {
			t := delayMaps[nodeInfo.IdentityIDShort][issuer]
			t.nodeQLen = qLen
			delayMaps[nodeInfo.IdentityIDShort][issuer] = t
		}
	}

	printResults(delayMaps)
	printMPSResults(mpsMaps)

	manaPercentage := fetchManaPercentage(clients[0])
	renderChart(delayMaps, manaPercentage)
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

func analyzeMPSDistribution(goshimmerAPI *client.GoShimmerAPI, endTime time.Time) map[string]MPSInfo {
	csvRes, err := goshimmerAPI.GetDiagnosticsMessages()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return calculateMPS(csvRes, endTime)
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

func calculateMPS(response *csv.Reader, endTime time.Time) map[string]MPSInfo {
	startTime := endTime.Add(timeWindow)
	nodeMSGCounterMap := make(map[string]int)
	nodeMPSMap := make(map[string]MPSInfo)
	messageInfos, _ := response.ReadAll()

	for _, msg := range messageInfos {
		arrivalTime := timestampFromString(msg[4])
		// ignore data that is issued before collectTime
		if arrivalTime.Before(startTime) || arrivalTime.After(endTime) {
			continue
		}

		issuer := msg[1]
		nodeMSGCounterMap[issuer]++
	}

	for nodeID, counter := range nodeMSGCounterMap {
		nodeMPSMap[nodeID] = MPSInfo{
			mps: float64(counter) / endTime.Sub(startTime).Seconds(),
		}
	}
	return nodeMPSMap
}

func fetchManaPercentage(goshimmerAPI *client.GoShimmerAPI) map[string]float64 {
	manaPercentageMap := make(map[string]float64)
	res, _ := goshimmerAPI.GetNHighestAccessMana(0)

	totalAccessMana := 0.0
	for _, node := range res.Nodes {
		totalAccessMana += node.Mana
	}

	for _, node := range res.Nodes {
		manaPercentageMap[node.ShortNodeID] = node.Mana / totalAccessMana
	}
	return manaPercentageMap
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
		title = fmt.Sprintf("%s %-30s %-15s", title, nodeID, "scheduled msgs")
	}
	fmt.Printf("%s\n\n", title)

	for _, issuer := range nodeOrder {
		row := fmt.Sprintf("%-15s", issuer)
		for _, nodeID := range nodeOrder {
			delayQLenstr := fmt.Sprintf("%v (Q size:%d)", time.Duration(delayMaps[nodeID][issuer].avgDelay)*time.Nanosecond,
				delayMaps[nodeID][issuer].nodeQLen)
			row = fmt.Sprintf("%s %-30s %-15d", row, delayQLenstr, delayMaps[nodeID][issuer].scheduledMsgs)
		}
		fmt.Println(row)
	}
}

func printMPSResults(mpsMaps map[string]map[string]MPSInfo) {
	fmt.Printf("The average mps of different issuers on different nodes:\n\n")

	var nodeOrder []string
	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for nodeID := range mpsMaps {
		nodeOrder = append(nodeOrder, nodeID)
		title = fmt.Sprintf("%s %-30s", title, nodeID)
	}
	fmt.Printf("%s\n\n", title)

	for _, issuer := range nodeOrder {
		row := fmt.Sprintf("%-15s", issuer)
		for _, nodeID := range nodeOrder {
			row = fmt.Sprintf("%s %-30f", row, mpsMaps[nodeID][issuer].mps)
		}
		fmt.Println(row)
	}
}
