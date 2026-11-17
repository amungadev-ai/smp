// Package model defines the Drevanta engine's domain types and their JSON wire
// shapes. These shapes are mirrored by the portal's TypeScript model.
package model

import "time"

const (
	// Project statuses
	StatusIdle      = "idle"
	StatusDeploying = "deploying"
	StatusRunning   = "running"
	StatusStopped   = "stopped"
	StatusFailed    = "failed"

	// Release statuses
	ReleaseQueued     = "queued"
	ReleaseDeploying  = "deploying"
	ReleaseHealthy    = "healthy"
	ReleaseUnhealthy  = "unhealthy"
	ReleaseFailed     = "failed"
	ReleaseRolledBack = "rolled_back"

	// Job kinds and statuses
	JobDeploy  = "deploy"
	JobRollback = "rollback"
	JobRestart = "restart"

	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"

	// Domain statuses
	DomainActive  = "active"
	DomainPending = "pending"
)

func Now() string { return time.Now().UTC().Format(time.RFC3339) }

type RuntimeInfo struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Version   string `json:"version"`
}

type ServerInfo struct {
	Hostname      string      `json:"hostname"`
	OS            string      `json:"os"`
	Kernel        string      `json:"kernel"`
	Arch          string      `json:"arch"`
	CPUCores      int         `json:"cpu_cores"`
	MemTotal      uint64      `json:"mem_total"`
	DiskTotal     uint64      `json:"disk_total"`
	DiskUsed      uint64      `json:"disk_used"`
	IP            string      `json:"ip"`
	UptimeS       int64       `json:"uptime_s"`
	Docker        RuntimeInfo `json:"docker"`
	Caddy         RuntimeInfo `json:"caddy"`
	EngineVersion string      `json:"engine_version"`
	MockMode      bool        `json:"mock_mode"`
}

type ReadinessCheck struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Image          string `json:"image"`
	Port           int    `json:"port"`
	Domain         string `json:"domain,omitempty"`
	HTTPS          bool   `json:"https,omitempty"`
	Status         string `json:"status"`
	Created        string `json:"created"`
	CurrentRelease string `json:"current_release,omitempty"`
	UpstreamPort int `json:"upstream_port,omitempty"`
	ReleaseCount   int    `json:"release_count"`
}

type Release struct {
	ID              string `json:"id"`
	ProjectID       string `json:"project_id"`
	Image           string `json:"image"`
	Tag             string `json:"tag"`
	Created         string `json:"created"`
	Status          string `json:"status"`
	HealthOK        *bool  `json:"health_ok,omitempty"`
	HealthLatencyMS int64  `json:"health_latency_ms,omitempty"`
	RolledBackTo    string `json:"rolled_back_to,omitempty"`
	HostPort int `json:"host_port,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

type JobLogLine struct {
	TS      string `json:"ts"`
	Message string `json:"message"`
}

// LogLine is one SSE frame of a container log stream.
type LogLine struct {
	TS      string `json:"ts,omitempty"`
	Stream  string `json:"stream"`
	Message string `json:"message"`
}

type Job struct {
	ID        string       `json:"id"`
	ProjectID string       `json:"project_id"`
	Kind      string       `json:"kind"`
	Status    string       `json:"status"`
	Created   string       `json:"created"`
	Started   string       `json:"started,omitempty"`
	Finished  string       `json:"finished,omitempty"`
	Error     string       `json:"error,omitempty"`
	ReleaseID string       `json:"release_id,omitempty"`
	Image string `json:"image,omitempty"`
	Log       []JobLogLine `json:"log"`
}

type Domain struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProjectID  string `json:"project_id,omitempty"`
	Scheme     string `json:"scheme"`
	Created    string `json:"created"`
	Status     string `json:"status"`
}

type State struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Releases []Release `json:"releases"`
	Jobs     []Job     `json:"jobs"`
	Domains  []Domain  `json:"domains"`
}
