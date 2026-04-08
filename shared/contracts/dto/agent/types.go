package agent

type RegisterRequest struct {
	AgentID   string `json:"agentId"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname"`
	Platform  string `json:"platform"`
	Mode      string `json:"mode"`
	ProcessID int64  `json:"processId"`
}

type HeartbeatRequest struct {
	AgentID   string `json:"agentId"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname"`
	Platform  string `json:"platform"`
	Mode      string `json:"mode"`
	ProcessID int64  `json:"processId"`
}
