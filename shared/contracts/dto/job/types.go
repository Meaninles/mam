package job

type Record struct {
	ID                 string      `json:"id"`
	Code               string      `json:"code"`
	LibraryID          *string     `json:"libraryId,omitempty"`
	JobFamily          string      `json:"jobFamily"`
	JobIntent          string      `json:"jobIntent"`
	RouteType          *string     `json:"routeType,omitempty"`
	Status             string      `json:"status"`
	Priority           string      `json:"priority"`
	Title              string      `json:"title"`
	Summary            string      `json:"summary"`
	SourceDomain       string      `json:"sourceDomain"`
	SourceRefID        *string     `json:"sourceRefId,omitempty"`
	SourceSnapshot     interface{} `json:"sourceSnapshot,omitempty"`
	ProgressPercent    float64     `json:"progressPercent"`
	SpeedBPS           *int64      `json:"speedBps,omitempty"`
	ETASeconds         *int        `json:"etaSeconds,omitempty"`
	TotalItems         int         `json:"totalItems"`
	SuccessItems       int         `json:"successItems"`
	FailedItems        int         `json:"failedItems"`
	SkippedItems       int         `json:"skippedItems"`
	IssueCount         int         `json:"issueCount"`
	LatestErrorCode    *string     `json:"latestErrorCode,omitempty"`
	LatestErrorMessage *string     `json:"latestErrorMessage,omitempty"`
	OutcomeSummary     *string     `json:"outcomeSummary,omitempty"`
	CreatedByType      string      `json:"createdByType"`
	CreatedByRef       *string     `json:"createdByRef,omitempty"`
	CreatedAt          string      `json:"createdAt"`
	StartedAt          *string     `json:"startedAt,omitempty"`
	FinishedAt         *string     `json:"finishedAt,omitempty"`
	CanceledAt         *string     `json:"canceledAt,omitempty"`
	UpdatedAt          string      `json:"updatedAt"`
}

type ItemRecord struct {
	ID                 string  `json:"id"`
	JobID              string  `json:"jobId"`
	ParentItemID       *string `json:"parentItemId,omitempty"`
	ItemKey            string  `json:"itemKey"`
	ItemType           string  `json:"itemType"`
	RouteType          *string `json:"routeType,omitempty"`
	Status             string  `json:"status"`
	Phase              *string `json:"phase,omitempty"`
	Title              string  `json:"title"`
	Summary            string  `json:"summary"`
	SourcePath         *string `json:"sourcePath,omitempty"`
	TargetPath         *string `json:"targetPath,omitempty"`
	ProgressPercent    float64 `json:"progressPercent"`
	SpeedBPS           *int64  `json:"speedBps,omitempty"`
	ETASeconds         *int    `json:"etaSeconds,omitempty"`
	BytesTotal         *int64  `json:"bytesTotal,omitempty"`
	BytesDone          *int64  `json:"bytesDone,omitempty"`
	AttemptCount       int     `json:"attemptCount"`
	IssueCount         int     `json:"issueCount"`
	LatestErrorCode    *string `json:"latestErrorCode,omitempty"`
	LatestErrorMessage *string `json:"latestErrorMessage,omitempty"`
	ResultSummary      *string `json:"resultSummary,omitempty"`
	StartedAt          *string `json:"startedAt,omitempty"`
	FinishedAt         *string `json:"finishedAt,omitempty"`
	CanceledAt         *string `json:"canceledAt,omitempty"`
	UpdatedAt          string  `json:"updatedAt"`
	CreatedAt          string  `json:"createdAt"`
}

type EventRecord struct {
	ID           string      `json:"id"`
	JobID        string      `json:"jobId"`
	JobItemID    *string     `json:"jobItemId,omitempty"`
	JobAttemptID *string     `json:"jobAttemptId,omitempty"`
	EventType    string      `json:"eventType"`
	Message      string      `json:"message"`
	Payload      interface{} `json:"payload,omitempty"`
	CreatedAt    string      `json:"createdAt"`
}

type ObjectLinkRecord struct {
	ID             string  `json:"id"`
	JobID          string  `json:"jobId"`
	JobItemID      *string `json:"jobItemId,omitempty"`
	LinkRole       string  `json:"linkRole"`
	ObjectType     string  `json:"objectType"`
	AssetID        *string `json:"assetId,omitempty"`
	AssetReplicaID *string `json:"assetReplicaId,omitempty"`
	DirectoryID    *string `json:"directoryId,omitempty"`
	MountID        *string `json:"mountId,omitempty"`
	StorageNodeID  *string `json:"storageNodeId,omitempty"`
	CreatedAt      string  `json:"createdAt"`
}

type Detail struct {
	Job   Record             `json:"job"`
	Items []ItemRecord       `json:"items"`
	Links []ObjectLinkRecord `json:"links"`
}

type ListResponse struct {
	Items    []Record `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type EventListResponse struct {
	Items []EventRecord `json:"items"`
}

type MutationResponse struct {
	Message string `json:"message"`
	Job     Record `json:"job"`
}

type CreateResponse struct {
	Message string `json:"message"`
	JobID   string `json:"jobId"`
	Job     Record `json:"job"`
}

type UpdatePriorityRequest struct {
	Priority string `json:"priority"`
}

type StreamEvent struct {
	EventID    string  `json:"eventId"`
	Topic      string  `json:"topic"`
	EventType  string  `json:"eventType"`
	JobID      string  `json:"jobId"`
	JobItemID  *string `json:"jobItemId,omitempty"`
	JobStatus  *string `json:"jobStatus,omitempty"`
	ItemStatus *string `json:"itemStatus,omitempty"`
	Message    string  `json:"message"`
	CreatedAt  string  `json:"createdAt"`
}
