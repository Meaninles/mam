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

type NasRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	AccessMode   string `json:"accessMode"`
	Username     string `json:"username"`
	PasswordHint string `json:"passwordHint"`
	LastTestAt   string `json:"lastTestAt,omitempty"`
	Status       string `json:"status"`
	Tone         string `json:"tone"`
	MountCount   int    `json:"mountCount"`
	Notes        string `json:"notes"`
}

type SaveNasNodeRequest struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
	Notes    string `json:"notes"`
}

type SaveNasNodeResponse struct {
	Message string    `json:"message"`
	Record  NasRecord `json:"record"`
}

type RunNasNodeConnectionTestRequest struct {
	IDs []string `json:"ids"`
}

type RunNasNodeConnectionTestResponse struct {
	Message string                 `json:"message"`
	Results []ConnectionTestResult `json:"results"`
}

type DeleteNasNodeResponse struct {
	Message string `json:"message"`
}

type CloudNodeRecord struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Vendor           string `json:"vendor"`
	AccessMethod     string `json:"accessMethod"`
	QRChannel        string `json:"qrChannel,omitempty"`
	MountPath        string `json:"mountPath"`
	TokenStatus      string `json:"tokenStatus"`
	Token            string `json:"token,omitempty"`
	AccountAlias     string `json:"accountAlias,omitempty"`
	LastAuthAt       string `json:"lastAuthAt,omitempty"`
	LastAuthResult   string `json:"lastAuthResult,omitempty"`
	LastErrorCode    string `json:"lastErrorCode,omitempty"`
	LastErrorMessage string `json:"lastErrorMessage,omitempty"`
	LastTestAt       string `json:"lastTestAt,omitempty"`
	Status           string `json:"status"`
	Tone             string `json:"tone"`
	MountCount       int    `json:"mountCount"`
	Notes            string `json:"notes"`
}

type SaveCloudNodeRequest struct {
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name"`
	Vendor       string              `json:"vendor"`
	AccessMethod string              `json:"accessMethod"`
	QRChannel    string              `json:"qrChannel,omitempty"`
	MountPath    string              `json:"mountPath"`
	Token        string              `json:"token"`
	QRSession    *CloudQRCodeSession `json:"qrSession,omitempty"`
	Notes        string              `json:"notes"`
}

type SaveCloudNodeResponse struct {
	Message string          `json:"message"`
	Record  CloudNodeRecord `json:"record"`
}

type RunCloudNodeConnectionTestRequest struct {
	IDs []string `json:"ids"`
}

type RunCloudNodeConnectionTestResponse struct {
	Message string                 `json:"message"`
	Results []ConnectionTestResult `json:"results"`
}

type DeleteCloudNodeResponse struct {
	Message string `json:"message"`
}

type CloudQRCodeSessionRequest struct {
	Channel string `json:"channel"`
}

type CloudQRCodeSession struct {
	UID          string `json:"uid"`
	Time         int64  `json:"time"`
	Sign         string `json:"sign"`
	QRCode       string `json:"qrcode"`
	Channel      string `json:"channel"`
	CodeVerifier string `json:"codeVerifier,omitempty"`
}

type CloudQRCodeStatusResponse struct {
	Status  string `json:"status"`
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
