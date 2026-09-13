package engine

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
)

// EngineClient defines the interface for communicating with Container Engine API.
type EngineClient interface {
	Ping(ctx context.Context) (*EngineInfo, error)
	ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error)
	InspectContainer(ctx context.Context, nameOrID string) (*ContainerInspect, error)
	CreateContainer(ctx context.Context, spec ContainerSpec) (string, error)
	StartContainer(ctx context.Context, nameOrID string) error
	StopContainer(ctx context.Context, nameOrID string, timeoutSeconds int) error
	RemoveContainer(ctx context.Context, nameOrID string, force bool) error
	RenameContainer(ctx context.Context, oldName, newName string) error

	ListVolumes(ctx context.Context) ([]VolumeInspect, error)
	InspectVolume(ctx context.Context, name string) (*VolumeInspect, error)
	CreateVolume(ctx context.Context, name string, labels map[string]string) (*VolumeInspect, error)
	RemoveVolume(ctx context.Context, name string, force bool) error

	ListNetworks(ctx context.Context) ([]NetworkInspect, error)
	InspectNetwork(ctx context.Context, nameOrID string) (*NetworkInspect, error)
	CreateNetwork(ctx context.Context, name string, labels map[string]string) (*NetworkInspect, error)
	RemoveNetwork(ctx context.Context, nameOrID string) error

	InspectImage(ctx context.Context, imageName string) (bool, error)
	GetInfo() *EngineInfo
}

// DetectSocket locates the active container engine socket path across OSes.
func DetectSocket(preferredEngine, explicitPath string) (engineType, socketPath string, err error) {
	if explicitPath != "" {
		if preferredEngine == "" {
			preferredEngine = "podman"
		}
		return preferredEngine, explicitPath, nil
	}

	if runtime.GOOS == "windows" {
		// Windows Named Pipes or Docker Desktop
		namedPipe := `\\.\pipe\docker_engine`
		return "docker", namedPipe, nil
	}

	// Linux / Darwin probing
	uid := os.Getuid()
	candidates := []struct {
		engine string
		path   string
	}{
		// Podman rootless socket (user specific)
		{"podman", fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)},
		{"podman", fmt.Sprintf("/var/run/user/%d/podman/podman.sock", uid)},
		// Docker rootless socket
		{"docker", fmt.Sprintf("/run/user/%d/docker.sock", uid)},
		{"docker", fmt.Sprintf("/var/run/user/%d/docker.sock", uid)},
		// Podman root socket
		{"podman", "/run/podman/podman.sock"},
		// Docker root socket
		{"docker", "/var/run/docker.sock"},
		{"docker", "/run/docker.sock"},
	}

	// Filter by preferredEngine if specified
	for _, c := range candidates {
		if preferredEngine != "" && c.engine != preferredEngine {
			continue
		}
		if fi, err := os.Stat(c.path); err == nil && (fi.Mode()&os.ModeSocket != 0) {
			return c.engine, c.path, nil
		}
	}

	// If no socket found by stat, return the top candidate based on preference
	if preferredEngine == "docker" {
		return "docker", "/var/run/docker.sock", nil
	}
	return "podman", fmt.Sprintf("/run/user/%d/podman/podman.sock", uid), nil
}

// NewEngineClient initializes a cross-platform EngineClient.
func NewEngineClient(engineType, socketPath string) (EngineClient, error) {
	if socketPath == "" {
		var err error
		engineType, socketPath, err = DetectSocket(engineType, "")
		if err != nil {
			return nil, err
		}
	}

	transport, err := createTransport(socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket transport for %s: %w", socketPath, err)
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	info := &EngineInfo{
		EngineType: engineType,
		SocketPath: socketPath,
		OSType:     runtime.GOOS,
	}

	return &RESTEngineAdapter{
		client:  httpClient,
		info:    info,
		baseURL: "http://localhost",
	}, nil
}
