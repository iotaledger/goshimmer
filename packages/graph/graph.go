package graph

import (
	"runtime"
	"sync"
)

type nodeID int32

type symbolTable map[string]nodeID

func (s symbolTable) getID(name string) nodeID {
	id, ok := s[name]
	if !ok {
		id = nodeID(len(s))
		s[name] = id
	}
	return id
}

// Graph contains nodes and a symbolTable of a graph.
type Graph struct {
	symbolTable
	nodes
}

// New returns a graph.
func New(ids []string) *Graph {
	g := &Graph{
		symbolTable: make(symbolTable, len(ids)),
		nodes:       make(nodes, len(ids)),
	}
	for index, id := range ids {
		g.nodes[index].ID = nodeID(index)
		g.symbolTable[id] = nodeID(index)
	}
	return g
}

// AddEdge adds an edge to the given graph.
func (g *Graph) AddEdge(a, b string) {
	aid := g.symbolTable.getID(a)
	bid := g.symbolTable.getID(b)

	g.nodes.AddEdge(aid, bid)
}

type node struct {
	ID nodeID

	// adjacent edges
	Adj []nodeID
}

func (n *node) add(adjNode *node) {
	for _, id := range n.Adj {
		if id == adjNode.ID {
			return
		}
	}
	n.Adj = append(n.Adj, adjNode.ID)
}

type nodes []node

func (nl nodes) get(id nodeID) *node {
	return &nl[id]
}

func (nl nodes) AddEdge(a, b nodeID) {
	an := nl.get(a)
	bn := nl.get(b)

	an.add(bn)
	bn.add(an)
}

// Diameter is the maximum length of a shortest path in the network
func (nl nodes) Diameter() int {
	cpus := runtime.GOMAXPROCS(0)
	numNodes := len(nl)
	nodesPerCPU := numNodes / cpus

	results := make([]int, cpus)
	wg := &sync.WaitGroup{}
	wg.Add(cpus)
	start := 0
	for cpu := 0; cpu < cpus; cpu++ {
		end := start + nodesPerCPU
		if cpu == cpus-1 {
			end = numNodes
		}

		go func(cpu int, start, end nodeID) {
			defer wg.Done()
			var diameter int
			q := &list{}
			depths := make([]bfsNode, numNodes)
			for id := start; id < end; id++ {
				// Need to reset the bfsData between runs
				for i := range depths {
					depths[i] = -1
				}

				df := nl.longestShortestPath(id, q, depths)
				if df > diameter {
					diameter = df
				}
			}
			results[cpu] = diameter
		}(cpu, nodeID(start), nodeID(end))
		start += nodesPerCPU
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

func (nl nodes) longestShortestPath(start nodeID, q *list, depths []bfsNode) int {
	n := nl.get(start)
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
				q.pushBack(nl.get(id))
			}
		}
	}

	return int(depths[n.ID])
}
