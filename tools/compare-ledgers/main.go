package main

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"

	"github.com/iotaledger/goshimmer/plugins/webapi/tools"

	//"github.com/iotaledger/goshimmer/plugins/webapi/message"

	"github.com/iotaledger/goshimmer/client"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cfgNodeURI = "nodes"
)

var (
	nodeLevelMembers []map[int][]tools.Approver
	bfsWg            sync.WaitGroup
	numberOfNodes    = 0
)

func init() {
	flag.StringSlice(cfgNodeURI, []string{"http://127.0.0.1:8080", "http://127.0.0.1:8085"}, "the API of the nodes")
}

func main() {

	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	numberOfNodes = len(viper.GetStringSlice(cfgNodeURI))
	nodeLevelMembers = make([]map[int][]tools.Approver, numberOfNodes)

	var goshimmerAPIs = make([]*client.GoShimmerAPI, numberOfNodes)
	for i, endpoint := range viper.GetStringSlice(cfgNodeURI) {
		goshimmerAPIs[i] = client.NewGoShimmerAPI(endpoint)
		if _, err := goshimmerAPIs[i].Info(); err != nil {
			fmt.Printf("cannot reach %s, error: %s\n", endpoint, err.Error())
			return
		}
	}

	for i, api := range goshimmerAPIs {
		ids, err := valueApprovers(api, payload.GenesisID.String())
		if err != nil {
			fmt.Printf("error: %s\n", err.Error())
			return
		}

		bfsWg.Add(1)
		go bfsValues(api, ids, i)
	}
	bfsWg.Wait()
	compareValues()
}

// valueApprovers fetches the approvers of the value object identified by the ID specified
func valueApprovers(api *client.GoShimmerAPI, ID string) (approvers []tools.Approver, err error) {
	approvers = nil
	res, err := api.ValueApprovers(ID)
	if err != nil {
		return
	}
	if res != nil && res.Error != "" {
		return
	}
	approvers = res.Approvers
	return
}

// bfsValues traverses the value tangle from genesis level-wise.
// for each level, we collect the value objects that we will compare level by level.
func bfsValues(api *client.GoShimmerAPI, roots []tools.Approver, nodeIndex int) {
	var q []tools.Approver
	levels := make(map[tools.Approver]int)
	levelMembers := make(map[int][]tools.Approver)

	prevLevel := -1
	for _, root := range roots {
		q = append(q, root)
		levels[root] = 0
	}

	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		currLevel := levels[v]
		members := levelMembers[currLevel]
		members = append(members, v)
		levelMembers[currLevel] = members
		if currLevel != prevLevel {
			prevLevel = currLevel
		}
		approvers, _ := valueApprovers(api, v.PayloadID)
		for _, w := range approvers {
			q = append(q, w)
			levels[w] = levels[v] + 1
		}
	}
	nodeLevelMembers[nodeIndex] = levelMembers
	bfsWg.Done()
}

func compareValues() {
	maxLevel := 0
	for _, levelMembers := range nodeLevelMembers {
		if len(levelMembers) > maxLevel {
			maxLevel = len(levelMembers)
		}
	}

	var redFlags []string
	for i := 0; i < maxLevel; i++ {
		fmt.Println("Level ", i)
		diff := make(map[tools.Approver][]int)
		for j, m := range nodeLevelMembers {
			for _, approver := range m[i] {
				l := diff[approver]
				l = append(l, j)
				diff[approver] = l
			}
		}
		var line = 0
		for k, v := range diff {
			line++
			fmt.Printf("%+v : %v\n", k, v)
			// if len(v) == numberOfNodes, then payload is the same on all nodes.
			if len(v) != numberOfNodes {
				redFlags = append(redFlags, fmt.Sprintf("level %d, line %d", i, line))
			}
		}
	}

	fmt.Println("Red flags: ", len(redFlags))
	for _, redFlag := range redFlags {
		fmt.Println(redFlag)
	}
}
