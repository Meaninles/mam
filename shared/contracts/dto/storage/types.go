package storage

type LocalNodeRecord struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	RootPath         string `json:"rootPath"`
	Enabled          bool   `json:"enabled"`
	HealthStatus     string `json:"healthStatus"`
	HealthTone       string `json:"healthTone"`
	LastCheckAt      string `json:"lastCheckAt"`
	CapacitySummary  string `json:"capacitySummary"`
	FreeSpaceSummary string `json:"freeSpaceSummary"`
	CapacityPercent  int    `json:"capacityPercent"`
	MountCount       int    `json:"mountCount"`
	Notes            string `json:"notes"`
}

type SaveLocalNodeRequest struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	RootPath string `json:"rootPath"`
	Notes    string `json:"notes"`
}

type SaveLocalNodeResponse struct {
	Message string          `json:"message"`
	Record  LocalNodeRecord `json:"record"`
}

type RunLocalNodeConnectionTestRequest struct {
	IDs []string `json:"ids"`
}

type RunLocalNodeConnectionTestResponse struct {
	Message string                 `json:"message"`
	Results []ConnectionTestResult `json:"results"`
}

type DeleteLocalNodeResponse struct {
	Message string `json:"message"`
}

type LocalFolderRecord struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	LibraryID        string   `json:"libraryId"`
	LibraryName      string   `json:"libraryName"`
	FolderType       string   `json:"folderType"`
	NodeID           string   `json:"nodeId"`
	NodeName         string   `json:"nodeName"`
	NodeRootPath     string   `json:"nodeRootPath"`
	RelativePath     string   `json:"relativePath"`
	Address          string   `json:"address"`
	MountMode        string   `json:"mountMode"`
	Enabled          bool     `json:"enabled"`
	ScanStatus       string   `json:"scanStatus"`
	ScanTone         string   `json:"scanTone"`
	LastScanAt       string   `json:"lastScanAt"`
	HeartbeatPolicy  string   `json:"heartbeatPolicy"`
	NextHeartbeatAt  string   `json:"nextHeartbeatAt"`
	CapacitySummary  string   `json:"capacitySummary"`
	FreeSpaceSummary string   `json:"freeSpaceSummary"`
	CapacityPercent  int      `json:"capacityPercent"`
	RiskTags         []string `json:"riskTags"`
	Badges           []string `json:"badges"`
	AuthStatus       string   `json:"authStatus"`
	AuthTone         string   `json:"authTone"`
	Notes            string   `json:"notes"`
}

type SaveLocalFolderRequest struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name"`
	LibraryID       string `json:"libraryId"`
	LibraryName     string `json:"libraryName"`
	NodeID          string `json:"nodeId"`
	MountMode       string `json:"mountMode"`
	HeartbeatPolicy string `json:"heartbeatPolicy"`
	RelativePath    string `json:"relativePath"`
	Notes           string `json:"notes"`
}

type SaveLocalFolderResponse struct {
	Message string            `json:"message"`
	Record  LocalFolderRecord `json:"record"`
}

type RunLocalFolderScanRequest struct {
	IDs []string `json:"ids"`
}

type RunLocalFolderScanResponse struct {
	Message string `json:"message"`
}

type ConnectionCheck struct {
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type ConnectionTestResult struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	OverallTone string            `json:"overallTone"`
	Summary     string            `json:"summary"`
	Checks      []ConnectionCheck `json:"checks"`
	Suggestion  string            `json:"suggestion,omitempty"`
	TestedAt    string            `json:"testedAt"`
}

type RunLocalFolderConnectionTestRequest struct {
	IDs []string `json:"ids"`
}

type RunLocalFolderConnectionTestResponse struct {
	Message string                 `json:"message"`
	Results []ConnectionTestResult `json:"results"`
}

type UpdateHeartbeatRequest struct {
	IDs             []string `json:"ids"`
	HeartbeatPolicy string   `json:"heartbeatPolicy"`
}

type UpdateHeartbeatResponse struct {
	Message string `json:"message"`
}

type ScanHistoryItem struct {
	ID         string `json:"id"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Status     string `json:"status"`
	Summary    string `json:"summary"`
	Trigger    string `json:"trigger"`
}

type LocalFolderScanHistoryResponse struct {
	ID    string            `json:"id"`
	Items []ScanHistoryItem `json:"items"`
}

type DeleteLocalFolderResponse struct {
	Message string `json:"message"`
}
