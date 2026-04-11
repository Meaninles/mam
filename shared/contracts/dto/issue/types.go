package issue

type SourceContext struct {
	TaskID        *string `json:"taskId,omitempty"`
	TaskTitle     *string `json:"taskTitle,omitempty"`
	TaskItemID    *string `json:"taskItemId,omitempty"`
	TaskItemTitle *string `json:"taskItemTitle,omitempty"`
	AssetID       *string `json:"assetId,omitempty"`
	EntryID       *string `json:"entryId,omitempty"`
	EndpointID    *string `json:"endpointId,omitempty"`
	EndpointLabel *string `json:"endpointLabel,omitempty"`
	Path          *string `json:"path,omitempty"`
	SourceLabel   *string `json:"sourceLabel,omitempty"`
	RouteLabel    *string `json:"routeLabel,omitempty"`
}

type ImpactSummary struct {
	AssetCount          int  `json:"assetCount"`
	ReplicaCount        int  `json:"replicaCount"`
	DirectoryCount      int  `json:"directoryCount"`
	EndpointCount       int  `json:"endpointCount"`
	BlocksStatusCommit  bool `json:"blocksStatusCommit"`
	BlocksTaskExecution bool `json:"blocksTaskExecution"`
}

type Capabilities struct {
	CanRetry            bool `json:"canRetry"`
	CanConfirm          bool `json:"canConfirm"`
	CanIgnore           bool `json:"canIgnore"`
	CanArchive          bool `json:"canArchive"`
	CanClearHistory     bool `json:"canClearHistory"`
	CanOpenTaskCenter   bool `json:"canOpenTaskCenter"`
	CanOpenFileCenter   bool `json:"canOpenFileCenter"`
	CanOpenStorageNodes bool `json:"canOpenStorageNodes"`
}

type HistoryRecord struct {
	ID            string `json:"id"`
	IssueID       string `json:"issueId"`
	Action        string `json:"action"`
	OperatorLabel string `json:"operatorLabel"`
	Result        string `json:"result"`
	CreatedAt     string `json:"createdAt"`
}

type Record struct {
	ID                   string          `json:"id"`
	Code                 string          `json:"code"`
	LibraryID            *string         `json:"libraryId,omitempty"`
	TaskID               *string         `json:"taskId,omitempty"`
	TaskItemID           *string         `json:"taskItemId,omitempty"`
	IssueCategory        string          `json:"issueCategory"`
	IssueType            string          `json:"issueType"`
	Nature               string          `json:"nature"`
	SourceDomain         string          `json:"sourceDomain"`
	Severity             string          `json:"severity"`
	Status               string          `json:"status"`
	Title                string          `json:"title"`
	Summary              string          `json:"summary"`
	AssetLabel           *string         `json:"assetLabel,omitempty"`
	ObjectLabel          string          `json:"objectLabel"`
	SuggestedAction      *string         `json:"suggestedAction,omitempty"`
	SuggestedActionLabel *string         `json:"suggestedActionLabel,omitempty"`
	Suggestion           *string         `json:"suggestion,omitempty"`
	Detail               *string         `json:"detail,omitempty"`
	OccurrenceCount      int             `json:"occurrenceCount"`
	CreatedAt            string          `json:"createdAt"`
	UpdatedAt            string          `json:"updatedAt"`
	ResolvedAt           *string         `json:"resolvedAt,omitempty"`
	ArchivedAt           *string         `json:"archivedAt,omitempty"`
	Source               SourceContext   `json:"source"`
	Impact               ImpactSummary   `json:"impact"`
	Capabilities         Capabilities    `json:"capabilities"`
	Histories            []HistoryRecord `json:"histories"`
}

type ListResponse struct {
	Items    []Record `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type ActionRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"`
}

type ActionResponse struct {
	Message string   `json:"message"`
	IDs     []string `json:"ids"`
}

type ClearHistoryRequest struct {
	IDs []string `json:"ids"`
}

type ClearHistoryResponse struct {
	Message string   `json:"message"`
	IDs     []string `json:"ids"`
}
