package runtime

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	AgentID      string
	Version      string
	Hostname     string
	Platform     string
	Mode         string
	ProcessID    int64
	Capabilities []string
	StartedAt    time.Time
}

type Summary struct {
	AgentID      string   `json:"agentId"`
	Version      string   `json:"version"`
	Hostname     string   `json:"hostname"`
	Platform     string   `json:"platform"`
	Mode         string   `json:"mode"`
	ProcessID    int64    `json:"processId"`
	Capabilities []string `json:"capabilities"`
	StartedAt    string   `json:"startedAt"`
}

func BuildSummary(options Options) (Summary, error) {
	if strings.TrimSpace(options.AgentID) == "" {
		return Summary{}, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(options.Version) == "" {
		return Summary{}, fmt.Errorf("version is required")
	}
	if strings.TrimSpace(options.Hostname) == "" {
		return Summary{}, fmt.Errorf("hostname is required")
	}
	if strings.TrimSpace(options.Platform) == "" {
		return Summary{}, fmt.Errorf("platform is required")
	}
	if strings.TrimSpace(options.Mode) == "" {
		return Summary{}, fmt.Errorf("mode is required")
	}
	if options.ProcessID <= 0 {
		return Summary{}, fmt.Errorf("process id must be greater than 0")
	}
	if options.StartedAt.IsZero() {
		return Summary{}, fmt.Errorf("startedAt is required")
	}

	return Summary{
		AgentID:      options.AgentID,
		Version:      options.Version,
		Hostname:     options.Hostname,
		Platform:     options.Platform,
		Mode:         options.Mode,
		ProcessID:    options.ProcessID,
		Capabilities: append([]string(nil), options.Capabilities...),
		StartedAt:    options.StartedAt.UTC().Format(time.RFC3339),
	}, nil
}

func DetectPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
