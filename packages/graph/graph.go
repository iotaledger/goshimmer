package graph

import (
	"runtime"
	"sync"
)

type nodeId int32

type SymbolTable map[string]nodeId

func (s SymbolTable) getId(name string) nodeId {
	id, ok := s[name]
	if !ok {
		id = nodeId(len(s))
		s[name] = id
	}
	return id
}

type Graph struct {
	SymbolTable
	Nodes
}

func New(IDs []string) *Graph {
	g := &Graph{
		SymbolTable: make(SymbolTable, len(IDs)),
		Nodes:       make(Nodes, len(IDs)),
	}
	for index, id := range IDs {
		g.Nodes[index].ID = nodeId(index)
		g.SymbolTable[id] = nodeId(index)
	}
	return g
}

func (g *Graph) AddEdge(a, b string) {
	aid := g.SymbolTable.getId(a)
	bid := g.SymbolTable.getId(b)

	g.Nodes.AddEdge(aid, bid)
}

type Node struct {
	ID nodeId

	// adjacent edges
	Adj []nodeId
}

func (n *Node) add(adjNode *Node) {
	for _, id := range n.Adj {
		if id == adjNode.ID {
			return
		}
	}
	n.Adj = append(n.Adj, adjNode.ID)
}

type Nodes []Node

func (nl Nodes) get(id nodeId) *Node {
	return &nl[id]
}

func (nl Nodes) AddEdge(a, b nodeId) {
	an := nl.get(a)
	bn := nl.get(b)

	an.add(bn)
	bn.add(an)
}

// diameter is the maximum length of a shortest path in the network
func (nl Nodes) Diameter() int {

	cpus := runtime.NumCPU()
	numNodes := len(nl)
	nodesPerCpu := numNodes / cpus

	results := make([]int, cpus)
	wg := &sync.WaitGroup{}
	wg.Add(cpus)
	start := 0
	for cpu := 0; cpu < cpus; cpu++ {
		end := start + nodesPerCpu
		if cpu == cpus-1 {
			end = numNodes
		}

		go func(cpu int, start, end nodeId) {
			defer wg.Done()
			var diameter int
			q := &list{}
			depths := make([]bfsNode, numNodes)
			for id := start; id < end; id++ {
				// Need to reset the bfsData between runs
				for i := range depths {
					depths[i] = -1
				}

				df := nl.longestShortestPath(nodeId(id), q, depths)
				if df > diameter {
					diameter = df
				}
			}
			results[cpu] = diameter
		}(cpu, nodeId(start), nodeId(end))
		start += nodesPerCpu
	}

	wg.Wait()

	diameter := 0
	for _, result := range results {
		if result > diameter {
			diameter = result
		}
	}
	return diameter
}

// bfs tracking data
type bfsNode int16

func (nodes Nodes) longestShortestPath(start nodeId, q *list, depths []bfsNode) int {

	n := nodes.get(start)
	depths[n.ID] = 0
	q.pushBack(n)

	for {
		newN := q.getHead()
		if newN == nil {
			break
		}
		n = newN

		for _, id := range n.Adj {
			if depths[id] == -1 {
				depths[id] = depths[n.ID] + 1
				q.pushBack(nodes.get(id))
			}
		}
	}

	return int(depths[n.ID])
}
