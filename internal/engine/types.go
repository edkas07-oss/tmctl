package engine

import "time"

// ContainerSummary represents brief container details from engine API.
type ContainerSummary struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	Command string            `json:"Command"`
	Created int64             `json:"Created"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Ports   []PortMappingJSON `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
}

// PortMappingJSON represents container port exposure in engine API.
type PortMappingJSON struct {
	IP          string `json:"IP,omitempty"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort,omitempty"`
	Type        string `json:"Type"`
}

// ContainerInspect represents full container state details.
type ContainerInspect struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status     string    `json:"Status"`
		Running    bool      `json:"Running"`
		Paused     bool      `json:"Paused"`
		Restarting bool      `json:"Restarting"`
		OOMKilled  bool      `json:"OOMKilled"`
		Dead       bool      `json:"Dead"`
		Pid        int       `json:"Pid"`
		ExitCode   int       `json:"ExitCode"`
		Error      string    `json:"Error"`
		StartedAt  time.Time `json:"StartedAt"`
		FinishedAt time.Time `json:"FinishedAt"`
		Health     *struct {
			Status string `json:"Status"`
		} `json:"Health,omitempty"`
	} `json:"State"`
	Config struct {
		Image  string            `json:"Image"`
		Env    []string          `json:"Env"`
		Cmd    []string          `json:"Cmd"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	NetworkSettings struct {
		IPAddress string `json:"IPAddress"`
		Ports     map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
			NetworkID string `json:"NetworkID"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// VolumeInspect represents volume info.
type VolumeInspect struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	CreatedAt  string            `json:"CreatedAt"`
	Labels     map[string]string `json:"Labels"`
	Scope      string            `json:"Scope"`
}

// NetworkInspect represents network info.
type NetworkInspect struct {
	ID       string            `json:"Id"`
	Name     string            `json:"Name"`
	Driver   string            `json:"Driver"`
	Scope    string            `json:"Scope"`
	Internal bool              `json:"Internal"`
	Labels   map[string]string `json:"Labels"`
}

// ContainerSpec represents declarative container creation parameters.
type ContainerSpec struct {
	Name          string
	Image         string
	Network       string
	NetworkAlias  string
	Ports         []PortBinding
	Volumes       []VolumeMount
	Env           []string
	Cmd           []string
	Entrypoint    []string
	RestartPolicy string
	User          string
	UsernsMode    string
	PullPolicy    string
	Labels        map[string]string
}

// PortBinding represents host-to-container port mapping.
type PortBinding struct {
	HostPort      int
	ContainerPort int
	Protocol      string // "tcp" or "udp"
}

// VolumeMount represents volume or bind mount.
type VolumeMount struct {
	Source   string
	Target   string
	ReadOnly bool
	Mode     string // e.g. "z", "ro,z", "ro"
	IsVolume bool   // true if named volume, false if host bind
}

// EngineInfo holds engine ping/version metadata.
type EngineInfo struct {
	EngineType string // "podman" or "docker"
	APIVersion string
	OSType     string
	SocketPath string
}

// EventActor represents event actor details in Docker/Podman events.
type EventActor struct {
	ID         string            `json:"ID"`
	Attributes map[string]string `json:"Attributes"`
}

// EventMessage represents a raw event payload streamed from the container engine.
type EventMessage struct {
	Type     string     `json:"Type"`
	Action   string     `json:"Action"`
	Status   string     `json:"status,omitempty"`
	Actor    EventActor `json:"Actor"`
	ID       string     `json:"id,omitempty"`
	From     string     `json:"from,omitempty"`
	Time     int64      `json:"time"`
	TimeNano int64      `json:"timeNano"`
	Name     string     `json:"Name,omitempty"`
}

// GetAction returns normalized action string (e.g. "die", "stop", "oom", "start").
func (e *EventMessage) GetAction() string {
	if e.Action != "" {
		return e.Action
	}
	if e.Status != "" {
		return e.Status
	}
	return ""
}

// GetContainerName returns container name from Attributes or Name field.
func (e *EventMessage) GetContainerName() string {
	if e.Actor.Attributes != nil {
		if name, ok := e.Actor.Attributes["name"]; ok && name != "" {
			return name
		}
	}
	if e.Name != "" {
		return e.Name
	}
	return ""
}

