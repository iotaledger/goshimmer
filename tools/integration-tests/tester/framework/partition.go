package framework

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// Partition represents a network partition.
// It contains its peers and the corresponding Pumba instances that block all traffic to peers in other partitions.
type Partition struct {
	name   string
	peers  []*Node
	pumbas []*DockerContainer
}

// Peers returns the peers in the partition.
func (p *Partition) Peers() []*Node {
	return p.peers
}

// PeerIDs returns the IDs of the peers in the partition
func (p *Partition) PeerIDs() []string {
	ids := make([]string, 0, len(p.peers))
	for _, peer := range p.peers {
		ids = append(ids, peer.ID().String())
	}
	return ids
}

// deletePartition deletes a partition, all its Pumba containers and creates logs for them.
func (p *Partition) deletePartition(ctx context.Context) error {
	// stop all pumba instances in parallel
	var eg errgroup.Group
	for _, pumba := range p.pumbas {
		pumba := pumba // capture range variable
		eg.Go(func() error {
			return pumba.Stop(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
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
