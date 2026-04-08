package runtime

type RuntimeStatusResponse struct {
	Status    string               `json:"status"`
	Service   ServiceRuntimeStatus `json:"service"`
	Database  ComponentStatus      `json:"database"`
	Migration MigrationStatus      `json:"migration"`
	Agents    []AgentRuntimeStatus `json:"agents"`
	Timestamp string               `json:"timestamp"`
}

type ServiceRuntimeStatus struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
}

type ComponentStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type MigrationStatus struct {
	Status         string `json:"status"`
	CurrentVersion int    `json:"currentVersion"`
	LatestVersion  int    `json:"latestVersion"`
}

type AgentRuntimeStatus struct {
	AgentID         string `json:"agentId"`
	Version         string `json:"version"`
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	Mode            string `json:"mode"`
	ProcessID       int64  `json:"processId"`
	Status          string `json:"status"`
	RegisteredAt    string `json:"registeredAt"`
	LastHeartbeatAt string `json:"lastHeartbeatAt"`
}
