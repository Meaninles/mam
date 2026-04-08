package agentregistry

import "time"

type Registration struct {
	AgentID   string `json:"agentId"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname"`
	Platform  string `json:"platform"`
	Mode      string `json:"mode"`
	ProcessID int64  `json:"processId"`
}

type Heartbeat struct {
	AgentID   string `json:"agentId"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname"`
	Platform  string `json:"platform"`
	Mode      string `json:"mode"`
	ProcessID int64  `json:"processId"`
}

type Agent struct {
	AgentID         string    `json:"agentId"`
	Version         string    `json:"version"`
	Hostname        string    `json:"hostname"`
	Platform        string    `json:"platform"`
	Mode            string    `json:"mode"`
	ProcessID       int64     `json:"processId"`
	RegisteredAt    time.Time `json:"registeredAt"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
}
