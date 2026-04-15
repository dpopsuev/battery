package battery

// Version and GitHash are set at build time via ldflags:
//
//	go build -ldflags "-X github.com/dpopsuev/battery.Version=v0.10.1 -X github.com/dpopsuev/battery.GitHash=abc123"
//
// Defaults to "dev" and "unknown" for development builds.
var (
	Version = "dev"
	GitHash = "unknown"
)
