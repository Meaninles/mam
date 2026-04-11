package jobs

import (
	"context"
	"time"

	jobdto "mare/shared/contracts/dto/job"
)

const (
	JobFamilyTransfer    = "TRANSFER"
	JobFamilyMaintenance = "MAINTENANCE"

	JobIntentImport          = "IMPORT"
	JobIntentReplicate       = "REPLICATE"
	JobIntentDeleteReplica   = "DELETE_REPLICA"
	JobIntentDeleteAsset     = "DELETE_ASSET"
	JobIntentScanDirectory   = "SCAN_DIRECTORY"
	JobIntentVerifyReplica   = "VERIFY_REPLICA"
	JobIntentVerifyAsset     = "VERIFY_ASSET"
	JobIntentExtractMetadata = "EXTRACT_METADATA"
	JobIntentConnectionTest  = "CONNECTION_TEST"

	StatusPending             = "PENDING"
	StatusQueued              = "QUEUED"
	StatusRunning             = "RUNNING"
	StatusPaused              = "PAUSED"
	StatusWaitingConfirmation = "WAITING_CONFIRMATION"
	StatusWaitingRetry        = "WAITING_RETRY"
	StatusPartialSuccess      = "PARTIAL_SUCCESS"
	StatusFailed              = "FAILED"
	StatusCompleted           = "COMPLETED"
	StatusCanceled            = "CANCELED"

	ItemStatusPending      = "PENDING"
	ItemStatusQueued       = "QUEUED"
	ItemStatusRunning      = "RUNNING"
	ItemStatusPaused       = "PAUSED"
	ItemStatusWaitingRetry = "WAITING_RETRY"
	ItemStatusSkipped      = "SKIPPED"
	ItemStatusFailed       = "FAILED"
	ItemStatusCompleted    = "COMPLETED"
	ItemStatusCanceled     = "CANCELED"

	PriorityLow    = "LOW"
	PriorityNormal = "NORMAL"
	PriorityHigh   = "HIGH"

	SourceDomainFileCenter   = "FILE_CENTER"
	SourceDomainImportCenter = "IMPORT_CENTER"
	SourceDomainStorageNodes = "STORAGE_NODES"
	SourceDomainSystemPolicy = "SYSTEM_POLICY"
	SourceDomainIssueCenter  = "ISSUE_CENTER"
	SourceDomainScheduled    = "SCHEDULED"

	CreatedByUser   = "USER"
	CreatedBySystem = "SYSTEM"
	CreatedByAgent  = "AGENT"

	ItemTypeAssetReplicaTransfer = "ASSET_REPLICA_TRANSFER"
	ItemTypeDirectoryScan        = "DIRECTORY_SCAN"
	ItemTypeReplicaVerify        = "REPLICA_VERIFY"
	ItemTypeMetadataExtract      = "ASSET_METADATA_EXTRACT"
	ItemTypeReplicaDelete        = "REPLICA_DELETE"
	ItemTypeAssetDeleteStep      = "ASSET_DELETE_STEP"
	ItemTypeConnectivityCheck    = "CONNECTIVITY_CHECK"

	ObjectTypeAsset        = "ASSET"
	ObjectTypeAssetReplica = "ASSET_REPLICA"
	ObjectTypeDirectory    = "DIRECTORY"
	ObjectTypeMount        = "MOUNT"
	ObjectTypeStorageNode  = "STORAGE_NODE"

	LinkRoleSubjectAsset      = "SUBJECT_ASSET"
	LinkRoleSubjectReplica    = "SUBJECT_REPLICA"
	LinkRoleSourceDirectory   = "SOURCE_DIRECTORY"
	LinkRoleTargetDirectory   = "TARGET_DIRECTORY"
	LinkRoleSourceMount       = "SOURCE_MOUNT"
	LinkRoleTargetMount       = "TARGET_MOUNT"
	LinkRoleSourceStorageNode = "SOURCE_STORAGE_NODE"
	LinkRoleTargetStorageNode = "TARGET_STORAGE_NODE"

	EventJobCreated         = "JOB_CREATED"
	EventJobQueued          = "JOB_QUEUED"
	EventJobStarted         = "JOB_STARTED"
	EventJobPaused          = "JOB_PAUSED"
	EventJobResumed         = "JOB_RESUMED"
	EventJobCanceled        = "JOB_CANCELED"
	EventJobRetried         = "JOB_RETRIED"
	EventJobCompleted       = "JOB_COMPLETED"
	EventJobFailed          = "JOB_FAILED"
	EventJobPartialSuccess  = "JOB_PARTIAL_SUCCESS"
	EventJobPriorityChanged = "JOB_PRIORITY_CHANGED"
	EventItemStarted        = "JOB_ITEM_STARTED"
	EventItemPaused         = "JOB_ITEM_PAUSED"
	EventItemResumed        = "JOB_ITEM_RESUMED"
	EventItemCompleted      = "JOB_ITEM_COMPLETED"
	EventItemFailed         = "JOB_ITEM_FAILED"
	EventItemCanceled       = "JOB_ITEM_CANCELED"
)

type CreateObjectLinkInput struct {
	LinkRole       string
	ObjectType     string
	AssetID        *string
	AssetReplicaID *string
	DirectoryID    *string
	MountID        *string
	StorageNodeID  *string
}

type CreateItemInput struct {
	ItemKey    string
	ItemType   string
	RouteType  *string
	Title      string
	Summary    string
	SourcePath *string
	TargetPath *string
	Links      []CreateObjectLinkInput
}

type CreateJobInput struct {
	LibraryID      *string
	JobFamily      string
	JobIntent      string
	RouteType      *string
	Title          string
	Summary        string
	SourceDomain   string
	SourceRefID    *string
	SourceSnapshot map[string]any
	Priority       string
	CreatedByType  string
	CreatedByRef   *string
	Items          []CreateItemInput
	Links          []CreateObjectLinkInput
}

type ListQuery struct {
	Page         int
	PageSize     int
	SearchText   string
	Status       string
	JobFamily    string
	SourceDomain string
	LibraryID    string
}

type ExecutionContext struct {
	Job       jobdto.Record
	Item      jobdto.ItemRecord
	JobLinks  []jobdto.ObjectLinkRecord
	ItemLinks []jobdto.ObjectLinkRecord
}

type ItemExecutor func(ctx context.Context, execution ExecutionContext) error

type jobRow struct {
	ID                 string
	Code               string
	LibraryID          *string
	JobFamily          string
	JobIntent          string
	RouteType          *string
	Status             string
	Priority           string
	Title              string
	Summary            string
	SourceDomain       string
	SourceRefID        *string
	SourceSnapshot     []byte
	ProgressPercent    float64
	SpeedBPS           *int64
	ETASeconds         *int
	TotalItems         int
	SuccessItems       int
	FailedItems        int
	SkippedItems       int
	IssueCount         int
	LatestErrorCode    *string
	LatestErrorMessage *string
	OutcomeSummary     *string
	CreatedByType      string
	CreatedByRef       *string
	CreatedAt          time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CanceledAt         *time.Time
	UpdatedAt          time.Time
}

type itemRow struct {
	ID                 string
	JobID              string
	ParentItemID       *string
	ItemKey            string
	ItemType           string
	RouteType          *string
	Status             string
	Phase              *string
	Title              string
	Summary            string
	SourcePath         *string
	TargetPath         *string
	ProgressPercent    float64
	SpeedBPS           *int64
	ETASeconds         *int
	BytesTotal         *int64
	BytesDone          *int64
	AttemptCount       int
	IssueCount         int
	LatestErrorCode    *string
	LatestErrorMessage *string
	ResultSummary      *string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CanceledAt         *time.Time
	UpdatedAt          time.Time
	CreatedAt          time.Time
}

type eventRow struct {
	ID           string
	JobID        string
	JobItemID    *string
	JobAttemptID *string
	EventType    string
	Message      string
	Payload      []byte
	CreatedAt    time.Time
}

type attemptRow struct {
	ID           string
	JobID        string
	JobItemID    *string
	AttemptNo    int
	Status       string
	WorkerType   string
	WorkerRef    *string
	ErrorCode    *string
	ErrorMessage *string
	StartedAt    time.Time
	FinishedAt   *time.Time
}
