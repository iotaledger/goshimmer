package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/client"
)

var (
	// only messages issued in the last timeWindow mins are taken into analysis
	timeWindow      = -10 * time.Minute
	nodeInfos       []*nodeInfo
	nameNodeInfoMap map[string]*nodeInfo
)

type nodeInfo struct {
	name   string
	apiURL string
	nodeID string
	client *client.GoShimmerAPI
	mpm    int
}

type mpsInfo struct {
	mps  float64
	msgs float64
}

type schedulingInfo struct {
	avgDelay      int64
	scheduledMsgs int
	nodeQLen      int
}

func main() {
	nodeInfos = []*nodeInfo{
		{
			name:   "master",
			apiURL: "http://127.0.0.1:8080",
			mpm:    274,
		},
	}
	nameNodeInfoMap = make(map[string]*nodeInfo, len(nodeInfos))
	bindGoShimmerAPIAndNodeID()

	// start spamming
	toggleSpammer(true)

	fmt.Println(time.Now())
	time.Sleep(11 * time.Minute)

	// start collecting metrics
	endTime := time.Now()
	delayMaps := make(map[string]map[string]schedulingInfo, len(nodeInfos))
	mpsMaps := make(map[string]map[string]mpsInfo, len(nodeInfos))
	for _, info := range nodeInfos {
		apiInfo, err := info.client.Info()
		if err != nil {
			fmt.Println(info.apiURL, "crashed")
			continue
		}
		delayMaps[info.nodeID] = analyzeSchedulingDelay(info.client, endTime)
		mpsMaps[info.nodeID] = analyzeMPSDistribution(info.client, endTime)
		// get node queue sizes
		for issuer, qLen := range apiInfo.Scheduler.NodeQueueSizes {
			t := delayMaps[info.nodeID][issuer]
			t.nodeQLen = qLen
			delayMaps[info.nodeID][issuer] = t
		}
	}

	// stop spamming
	toggleSpammer(false)

	printResults(delayMaps)
	printMPSResults(mpsMaps)
	printStoredMsgsPercentage(mpsMaps)

	manaPercentage := fetchManaPercentage(nodeInfos[0].client)
	renderChart(delayMaps, manaPercentage)
}

func bindGoShimmerAPIAndNodeID() {
	for _, info := range nodeInfos {
		// create GoShimmer API
		api := client.NewGoShimmerAPI(info.apiURL, client.WithHTTPClient(http.Client{Timeout: 1800 * time.Second}))
		// get short node ID
		nodeInfo, err := api.Info()
		if err != nil {
			fmt.Println(api.BaseURL(), "crashed")
			continue
		}
		info.nodeID = nodeInfo.IdentityIDShort
		info.client = api

		nameNodeInfoMap[info.name] = info
	}
}

func toggleSpammer(enabled bool) {
	for _, info := range nodeInfos {
		if enabled && info.mpm <= 0 {
			continue
		}

		resp, err := info.client.ToggleSpammer(enabled, info.mpm)
		if err != nil {
			panic(err)
		}
		// debug logging
		if enabled {
			fmt.Println(info.name, "spamming at", info.mpm, resp)
		} else {
			fmt.Println(info.name, "stop spamming")
		}
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

func analyzeMPSDistribution(goshimmerAPI *client.GoShimmerAPI, endTime time.Time) map[string]mpsInfo {
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

func calculateMPS(response *csv.Reader, endTime time.Time) map[string]mpsInfo {
	startTime := endTime.Add(timeWindow)
	nodeMSGCounterMap := make(map[string]int)
	nodeMPSMap := make(map[string]mpsInfo)
	messageInfos, _ := response.ReadAll()
	totalMsgFromStart := 0
	storedMsgFromStart := make(map[string]int)

	for _, msg := range messageInfos {
		issuer := msg[1]
		totalMsgFromStart++
		storedMsgFromStart[issuer]++

		arrivalTime := timestampFromString(msg[4])
		// ignore data that is issued before collectTime
		if arrivalTime.Before(startTime) || arrivalTime.After(endTime) {
			continue
		}

		nodeMSGCounterMap[issuer]++
	}

	for nodeID, counter := range nodeMSGCounterMap {
		nodeMPSMap[nodeID] = mpsInfo{
			mps:  float64(counter) / endTime.Sub(startTime).Seconds(),
			msgs: float64(storedMsgFromStart[nodeID]) / float64(totalMsgFromStart),
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

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-30s %-15s", title, info.name, "scheduled msgs")
	}
	fmt.Printf("%s\n\n", title)

	for _, issuer := range nodeInfos {
		row := fmt.Sprintf("%-15s", issuer.name)
		issuerID := issuer.nodeID
		for _, node := range nodeInfos {
			nodeID := node.nodeID
			delayQLenstr := fmt.Sprintf("%v (Q size:%d)",
				time.Duration(delayMaps[nodeID][issuerID].avgDelay)*time.Nanosecond,
				delayMaps[nodeID][issuerID].nodeQLen)
			row = fmt.Sprintf("%s %-30s %-15d", row, delayQLenstr, delayMaps[nodeID][issuerID].scheduledMsgs)
		}
		fmt.Println(row)
	}
	fmt.Printf("\n")
}

func printMPSResults(mpsMaps map[string]map[string]mpsInfo) {
	fmt.Printf("The average mps of different issuers on different nodes:\n\n")

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-30s", title, info.name)
	}
	fmt.Printf("%s\n\n", title)

	for _, issuer := range nodeInfos {
		row := fmt.Sprintf("%-15s", issuer.name)
		for _, node := range nodeInfos {
			row = fmt.Sprintf("%s %-30f", row, mpsMaps[node.nodeID][issuer.nodeID].mps)
		}
		fmt.Println(row)
	}
	fmt.Printf("\n")
}

func printStoredMsgsPercentage(mpsMaps map[string]map[string]mpsInfo) {
	fmt.Printf("The proportion of msgs from different issuers on different nodes:\n\n")

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-30s", title, info.name)
	}
	fmt.Printf("%s\n\n", title)

	for _, issuer := range nodeInfos {
		row := fmt.Sprintf("%-15s", issuer.name)
		for _, node := range nodeInfos {
			row = fmt.Sprintf("%s %-30f", row, mpsMaps[node.nodeID][issuer.nodeID].msgs)
		}
		fmt.Println(row)
	}
}
