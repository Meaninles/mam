package importing

type LibraryOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DashboardResponse struct {
	Libraries       []LibraryOption        `json:"libraries"`
	Devices         []DeviceSessionRecord  `json:"devices"`
	Drafts          []DraftRecord          `json:"drafts"`
	Reports         []ReportSnapshot       `json:"reports"`
	TargetEndpoints []TargetEndpointRecord `json:"targetEndpoints"`
}

type CapacitySummary struct {
	Total       string `json:"total"`
	Available   string `json:"available"`
	UsedPercent int    `json:"usedPercent"`
}

type TargetEndpointRecord struct {
	ID             string `json:"id"`
	EndpointID     string `json:"endpointId"`
	LibraryID      string `json:"libraryId"`
	Label          string `json:"label"`
	Type           string `json:"type"`
	Writable       bool   `json:"writable"`
	AvailableSpace string `json:"availableSpace"`
	StatusLabel    string `json:"statusLabel"`
	Tone           string `json:"tone"`
}

type DeviceSessionRecord struct {
	ID                        string          `json:"id"`
	DeviceKey                 string          `json:"deviceKey"`
	DeviceLabel               string          `json:"deviceLabel"`
	DeviceType                string          `json:"deviceType"`
	LibraryID                 *string         `json:"libraryId,omitempty"`
	MountPath                 string          `json:"mountPath"`
	ConnectedAt               string          `json:"connectedAt"`
	ConnectedAtSortKey        int64           `json:"connectedAtSortKey"`
	LastSeenAt                string          `json:"lastSeenAt"`
	CapacitySummary           CapacitySummary `json:"capacitySummary"`
	ScanStatus                string          `json:"scanStatus"`
	SessionStatus             string          `json:"sessionStatus"`
	ActiveDraftID             *string         `json:"activeDraftId,omitempty"`
	LatestReportID            *string         `json:"latestReportId,omitempty"`
	IssueIDs                  []string        `json:"issueIds"`
	DuplicateCount            int             `json:"duplicateCount"`
	ExceptionCount            int             `json:"exceptionCount"`
	Description               string          `json:"description"`
	AvailableTargetEndpointIDs []string       `json:"availableTargetEndpointIds"`
}

type PrecheckItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type PrecheckSummary struct {
	BlockingCount int    `json:"blockingCount"`
	RiskCount     int    `json:"riskCount"`
	PassedCount   int    `json:"passedCount"`
	UpdatedAt     string `json:"updatedAt"`
	Checks        struct {
		SourceReadable string `json:"sourceReadable"`
		TargetWritable string `json:"targetWritable"`
		CapacityReady  string `json:"capacityReady"`
		PathConflict   string `json:"pathConflict"`
		DeviceOnline   string `json:"deviceOnline"`
		ExecutorReady  string `json:"executorReady"`
	} `json:"checks"`
	Items []PrecheckItem `json:"items"`
}

type DraftRecord struct {
	ID               string          `json:"id"`
	DeviceSessionID  string          `json:"deviceSessionId"`
	LibraryID        *string         `json:"libraryId,omitempty"`
	SelectedFileIDs  []string        `json:"selectedFileIds"`
	TargetEndpointIDs []string       `json:"targetEndpointIds"`
	TargetStrategy   string          `json:"targetStrategy"`
	PrecheckSummary  PrecheckSummary `json:"precheckSummary"`
	LastEditedAt     string          `json:"lastEditedAt"`
	HasBlockingIssues bool           `json:"hasBlockingIssues"`
	Status           string          `json:"status"`
}

type BrowserNodeRecord struct {
	ID               string   `json:"id"`
	DeviceSessionID  string   `json:"deviceSessionId"`
	EntryType        string   `json:"entryType"`
	Name             string   `json:"name"`
	RelativePath     string   `json:"relativePath"`
	FileKind         string   `json:"fileKind"`
	Size             *string  `json:"size,omitempty"`
	ModifiedAt       string   `json:"modifiedAt"`
	IsHidden         bool     `json:"isHidden"`
	HasChildren      bool     `json:"hasChildren"`
	TargetEndpointIDs []string `json:"targetEndpointIds"`
}

type BrowseSessionResponse struct {
	SessionID    string             `json:"sessionId"`
	CurrentPath  string             `json:"currentPath"`
	Items        []BrowserNodeRecord `json:"items"`
	Total        int                `json:"total"`
	Limit        int                `json:"limit"`
	Offset       int                `json:"offset"`
	HasMore      bool               `json:"hasMore"`
}

type ReportTargetSummary struct {
	EndpointID      string `json:"endpointId"`
	Label           string `json:"label"`
	Status          string `json:"status"`
	SuccessCount    int    `json:"successCount"`
	FailedCount     int    `json:"failedCount"`
	TransferredSize string `json:"transferredSize"`
}

type ReportSnapshot struct {
	ID              string                `json:"id"`
	DeviceSessionID string                `json:"deviceSessionId"`
	LibraryID       *string               `json:"libraryId,omitempty"`
	TaskID          string                `json:"taskId"`
	Title           string                `json:"title"`
	Status          string                `json:"status"`
	SubmittedAt     string                `json:"submittedAt"`
	FinishedAt      *string               `json:"finishedAt,omitempty"`
	SuccessCount    int                   `json:"successCount"`
	FailedCount     int                   `json:"failedCount"`
	PartialCount    int                   `json:"partialCount"`
	VerifySummary   string                `json:"verifySummary"`
	TargetSummaries []ReportTargetSummary `json:"targetSummaries"`
	IssueIDs        []string              `json:"issueIds"`
	LatestUpdatedAt string                `json:"latestUpdatedAt"`
	FileCount       int                   `json:"fileCount"`
	Note            *string               `json:"note,omitempty"`
}

type SetDraftLibraryRequest struct {
	LibraryID string `json:"libraryId"`
}

type SetSourceTargetsRequest struct {
	EntryType         string   `json:"entryType"`
	Name              string   `json:"name"`
	RelativePath      string   `json:"relativePath"`
	TargetEndpointIDs []string `json:"targetEndpointIds"`
}

type RefreshPrecheckRequest struct {
	DraftID string `json:"draftId"`
}

type SaveDraftRequest struct {
	DraftID string `json:"draftId"`
}

type SubmitResponse struct {
	Message  string         `json:"message"`
	ReportID string         `json:"reportId"`
	Report   ReportSnapshot `json:"report"`
}

type MutationResponse struct {
	Message string `json:"message"`
}

type SourceDescriptor struct {
	DeviceKey      string         `json:"deviceKey"`
	SourceType     string         `json:"sourceType"`
	DeviceLabel    string         `json:"deviceLabel"`
	DeviceType     string         `json:"deviceType"`
	SourcePath     string         `json:"sourcePath"`
	MountPath      string         `json:"mountPath"`
	VolumeName     *string        `json:"volumeName,omitempty"`
	FileSystem     *string        `json:"fileSystem,omitempty"`
	CapacityBytes  *int64         `json:"capacityBytes,omitempty"`
	AvailableBytes *int64         `json:"availableBytes,omitempty"`
	ConnectedAt    string         `json:"connectedAt"`
	LastSeenAt     string         `json:"lastSeenAt"`
	SourceSnapshot map[string]any `json:"sourceSnapshot,omitempty"`
}

type DiscoverSourcesResponse struct {
	Sources []SourceDescriptor `json:"sources"`
}

type BrowseRequest struct {
	SourcePath   string  `json:"sourcePath"`
	RelativePath *string `json:"relativePath,omitempty"`
	Limit        int     `json:"limit,omitempty"`
	Offset       int     `json:"offset,omitempty"`
}

type BrowseEntry struct {
	EntryType    string  `json:"entryType"`
	RelativePath string  `json:"relativePath"`
	Name         string  `json:"name"`
	Extension    *string `json:"extension,omitempty"`
	FileKind     string  `json:"fileKind"`
	SizeBytes    *int64  `json:"sizeBytes,omitempty"`
	ModifiedAt   string  `json:"modifiedAt"`
	IsHidden     bool    `json:"isHidden"`
	HasChildren  bool    `json:"hasChildren"`
}

type BrowseResponse struct {
	Entries     []BrowseEntry `json:"entries"`
	Total       int           `json:"total"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
	HasMore     bool          `json:"hasMore"`
	ScannedAt   string        `json:"scannedAt"`
}

type ExecuteImportTarget struct {
	TargetID         string  `json:"targetId"`
	NodeType         string  `json:"nodeType"`
	PhysicalPath     string  `json:"physicalPath"`
	Username         *string `json:"username,omitempty"`
	Password         *string `json:"password,omitempty"`
	PreserveMtime    bool    `json:"preserveMtime"`
}

type ExecuteImportRequest struct {
	SourcePath string                `json:"sourcePath"`
	Targets    []ExecuteImportTarget `json:"targets"`
}

type ExecuteImportTargetResult struct {
	TargetID      string `json:"targetId"`
	PhysicalPath  string `json:"physicalPath"`
	BytesWritten  int64  `json:"bytesWritten"`
	ModifiedAt    string `json:"modifiedAt"`
}

type ExecuteImportResponse struct {
	Targets []ExecuteImportTargetResult `json:"targets"`
}
