package framework

import (
	"fmt"

	"github.com/drand/drand/core"
)

// Drand represents a drand node (committe member) inside the Docker network
type Drand struct {
	// name of the drand instance, Docker container and hostname
	name string

	// Web API of this drand node
	*core.Client

	// the DockerContainer that this peer is running in
	*DockerContainer
}

// newDrand creates a new instance of Drand with the given information.
func newDrand(name string, dockerContainer *DockerContainer) *Drand {
	return &Drand{
		name:            name,
		Client:          core.NewGrpcClient(),
		DockerContainer: dockerContainer,
	}
}

func (d *Drand) String() string {
	return fmt.Sprintf("Drand:{%s}", d.name)
}
