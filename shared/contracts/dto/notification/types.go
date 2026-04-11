package notification

type JumpParams struct {
	Kind         string  `json:"kind"`
	IssueID      *string `json:"issueId,omitempty"`
	TaskID       *string `json:"taskId,omitempty"`
	TaskItemID   *string `json:"taskItemId,omitempty"`
	LibraryID    *string `json:"libraryId,omitempty"`
	EndpointID   *string `json:"endpointId,omitempty"`
	FileNodeID   *string `json:"fileNodeId,omitempty"`
	Path         *string `json:"path,omitempty"`
	SourceDomain *string `json:"sourceDomain,omitempty"`
	Label        *string `json:"label,omitempty"`
}

type Source struct {
	SourceDomain      *string `json:"sourceDomain,omitempty"`
	IssueCategory     *string `json:"issueCategory,omitempty"`
	IssueNature       *string `json:"issueNature,omitempty"`
	IssueSourceDomain *string `json:"issueSourceDomain,omitempty"`
	TaskID            *string `json:"taskId,omitempty"`
	TaskItemID        *string `json:"taskItemId,omitempty"`
	FileNodeID        *string `json:"fileNodeId,omitempty"`
	EndpointID        *string `json:"endpointId,omitempty"`
	Path              *string `json:"path,omitempty"`
	SourceLabel       *string `json:"sourceLabel,omitempty"`
	RouteLabel        *string `json:"routeLabel,omitempty"`
}

type Capabilities struct {
	CanMarkRead         bool `json:"canMarkRead"`
	CanOpenIssueCenter  bool `json:"canOpenIssueCenter"`
	CanOpenTaskCenter   bool `json:"canOpenTaskCenter"`
	CanOpenFileCenter   bool `json:"canOpenFileCenter"`
	CanOpenStorageNodes bool `json:"canOpenStorageNodes"`
	CanOpenImportCenter bool `json:"canOpenImportCenter"`
}

type Record struct {
	ID                string       `json:"id"`
	Kind              string       `json:"kind"`
	SourceType        string       `json:"sourceType"`
	SourceID          string       `json:"sourceId"`
	JobID             *string      `json:"jobId,omitempty"`
	IssueID           *string      `json:"issueId,omitempty"`
	LibraryID         *string      `json:"libraryId,omitempty"`
	LifecycleStatus   string       `json:"lifecycleStatus"`
	DefaultTargetKind string       `json:"defaultTargetKind"`
	Title             string       `json:"title"`
	Summary           string       `json:"summary"`
	Severity          string       `json:"severity"`
	ObjectLabel       string       `json:"objectLabel"`
	CreatedAt         string       `json:"createdAt"`
	UpdatedAt         string       `json:"updatedAt"`
	Source            Source       `json:"source"`
	Capabilities      Capabilities `json:"capabilities"`
	JumpParams        JumpParams   `json:"jumpParams"`
}

type ListResponse struct {
	Items    []Record `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type StreamEvent struct {
	EventID         string  `json:"eventId"`
	Topic           string  `json:"topic"`
	EventType       string  `json:"eventType"`
	NotificationID  string  `json:"notificationId"`
	LifecycleStatus *string `json:"lifecycleStatus,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}
