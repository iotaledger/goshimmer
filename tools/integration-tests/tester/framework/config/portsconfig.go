package config

type GoShimmerPort int

const (
	WebApiPort    GoShimmerPort = 8080
	DashboardPort               = 8081
	DebugPort                   = 4000
)

var GoShimmerPorts = []GoShimmerPort{
	WebApiPort,
	DashboardPort,
	DebugPort,
}
