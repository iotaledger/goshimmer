package config

type GoShimmerPort int

const (
	WebApiPort    GoShimmerPort = 8080
	DashboardPort GoShimmerPort = 8081
	DagVizPort    GoShimmerPort = 8061
	DebugPort     GoShimmerPort = 40000
)

var GoShimmerPorts = []GoShimmerPort{
	WebApiPort,
	DashboardPort,
	DagVizPort,
	DebugPort,
}
