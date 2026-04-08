package health

type LivezResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

type ReadyzResponse struct {
	Status    string          `json:"status"`
	Service   ComponentStatus `json:"service"`
	Database  ComponentStatus `json:"database"`
	Migration MigrationStatus `json:"migration"`
	Version   string          `json:"version"`
	Timestamp string          `json:"timestamp"`
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
