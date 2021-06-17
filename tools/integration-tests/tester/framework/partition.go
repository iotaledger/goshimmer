package framework

import (
	"context"
	"fmt"
)

// Partition represents a network partition.
// It contains its peers and the corresponding Pumba instances that block all traffic to peers in other partitions.
type Partition struct {
	name     string
	peers    []*Node
	peersMap map[string]*Node
	pumbas   []*DockerContainer
}

// Peers returns the partition's peers.
func (p *Partition) Peers() []*Node {
	return p.peers
}

// PeersMap returns the partition's peers map.
func (p *Partition) PeersMap() map[string]*Node {
	return p.peersMap
}

// deletePartition deletes a partition, all its Pumba containers and creates logs for them.
func (p *Partition) deletePartition(ctx context.Context) error {
	// stop containers
	for _, pumba := range p.pumbas {
		err := pumba.Stop(ctx)
		if err != nil {
			return err
		}
	}

	// retrieve logs
	for i, pumba := range p.pumbas {
		logs, err := pumba.Logs(ctx)
		if err != nil {
			return err
		}
		err = createLogFile(fmt.Sprintf("%s%s", p.name, p.peers[i].Name()), logs)
		if err != nil {
			return err
		}
	}

	for _, pumba := range p.pumbas {
		err := pumba.Remove(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
