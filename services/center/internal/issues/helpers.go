package issues

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	issuedto "mare/shared/contracts/dto/issue"
)

func (s *Service) loadHistories(ctx context.Context, issueID string) ([]issuedto.HistoryRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, issue_id, event_type, operator_label, message, created_at
		FROM issue_events
		WHERE issue_id = $1
		ORDER BY sequence_no ASC
	`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]issuedto.HistoryRecord, 0)
	for rows.Next() {
		var id string
		var linkedIssueID string
		var eventType string
		var operatorLabel *string
		var message *string
		var createdAt time.Time
		if err := rows.Scan(&id, &linkedIssueID, &eventType, &operatorLabel, &message, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, issuedto.HistoryRecord{
			ID:            id,
			IssueID:       linkedIssueID,
			Action:        historyActionLabel(eventType),
			OperatorLabel: coalesceText(operatorLabel),
			Result:        coalesceText(message),
			CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		})
	}
	return items, rows.Err()
}

func mapIssueRow(row issueRow, histories []issuedto.HistoryRecord) issuedto.Record {
	var source issuedto.SourceContext
	var impact issuedto.ImpactSummary
	if len(row.SourceSnapshot) > 0 {
		_ = json.Unmarshal(row.SourceSnapshot, &source)
	}
	if len(row.ImpactSnapshot) > 0 {
		_ = json.Unmarshal(row.ImpactSnapshot, &impact)
	}
	return issuedto.Record{
		ID:                   row.ID,
		Code:                 row.Code,
		LibraryID:            row.LibraryID,
		TaskID:               source.TaskID,
		TaskItemID:           source.TaskItemID,
		IssueCategory:        row.IssueCategory,
		IssueType:            row.IssueType,
		Nature:               row.Nature,
		SourceDomain:         row.SourceDomain,
		Severity:             row.Severity,
		Status:               row.Status,
		Title:                row.Title,
		Summary:              row.Summary,
		AssetLabel:           row.AssetLabel,
		ObjectLabel:          row.ObjectLabel,
		SuggestedAction:      row.SuggestedAction,
		SuggestedActionLabel: row.SuggestedActionLabel,
		Suggestion:           row.Suggestion,
		Detail:               row.Detail,
		CreatedAt:            row.FirstDetectedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            row.UpdatedAt.UTC().Format(time.RFC3339),
		ResolvedAt:           formatOptionalRFC3339(row.ResolvedAt),
		ArchivedAt:           formatOptionalRFC3339(row.ArchivedAt),
		Source:               source,
		Impact:               impact,
		Capabilities:         deriveCapabilities(row.Status, source),
		Histories:            histories,
	}
}

func (s *Service) insertIssueEvent(ctx context.Context, tx pgx.Tx, issueID string, input issueEventInput) error {
	var nextSequence int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_no), 0) + 1
		FROM issue_events
		WHERE issue_id = $1
	`, issueID).Scan(&nextSequence); err != nil {
		return err
	}

	payloadJSON, err := marshalNullableJSON(input.Payload)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO issue_events (
			id, issue_id, sequence_no, event_type, action_key, from_status, to_status, actor_type, actor_ref_id, operator_label, message, payload, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`, buildCode("issue-event-id", input.CreatedAt), issueID, nextSequence, input.EventType, input.ActionKey, input.FromStatus, input.ToStatus, input.ActorType, input.ActorRefID, input.OperatorLabel, input.Message, payloadJSON, input.CreatedAt)
	return err
}

func (s *Service) insertIssueLink(ctx context.Context, tx pgx.Tx, issueID string, link issueObjectLink, createdAt time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO issue_object_links (
			id, issue_id, link_role, object_type, job_id, job_item_id, asset_id, asset_replica_id, directory_id, mount_id, storage_node_id,
			external_ref_type, external_ref_id, object_label, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			$12, $13, $14, $15
		)
	`, buildCode("issue-link-id", createdAt), issueID, link.LinkRole, link.ObjectType, link.JobID, link.JobItemID, link.AssetID, link.AssetReplicaID, link.DirectoryID, link.MountID, link.StorageNodeID,
		link.ExternalRefType, link.ExternalRefID, link.ObjectLabel, createdAt)
	return err
}

func deriveCapabilities(status string, source issuedto.SourceContext) issuedto.Capabilities {
	historical := isHistoricalStatus(status)
	active := !historical && status != StatusResolved
	return issuedto.Capabilities{
		CanRetry:            active && source.TaskID != nil,
		CanConfirm:          active,
		CanIgnore:           active,
		CanArchive:          historical || status == StatusResolved,
		CanClearHistory:     historical || status == StatusResolved,
		CanOpenTaskCenter:   source.TaskID != nil,
		CanOpenFileCenter:   source.EntryID != nil,
		CanOpenStorageNodes: source.EndpointID != nil,
	}
}

func deriveSourceDomain(job jobSnapshot) string {
	if job.JobFamily == "TRANSFER" {
		return SourceDomainTransferJob
	}
	switch job.SourceDomain {
	case "FILE_CENTER":
		return SourceDomainFileCenter
	case "STORAGE_NODES":
		return SourceDomainStorage
	case "IMPORT_CENTER":
		return SourceDomainImport
	case "SYSTEM_POLICY", "ISSUE_CENTER", "SCHEDULED":
		return SourceDomainGovernance
	default:
		return SourceDomainMaintenanceJob
	}
}

func deriveIssueCategory(job jobSnapshot, item jobItemSnapshot) string {
	haystack := strings.ToLower(coalesceText(item.LatestErrorMessage) + " " + coalesceText(job.LatestErrorMessage))
	if strings.Contains(haystack, "冲突") || strings.Contains(haystack, "已存在") {
		return CategoryConflict
	}
	switch job.JobIntent {
	case "REPLICATE", "IMPORT", "DELETE_REPLICA":
		return CategoryTransfer
	case "VERIFY_REPLICA", "VERIFY_ASSET":
		return CategoryVerify
	case "CONNECTION_TEST":
		return CategoryNodePermission
	case "DELETE_ASSET":
		return CategoryCleanup
	default:
		return CategoryScanParse
	}
}

func deriveIssueType(job jobSnapshot, item jobItemSnapshot) string {
	if strings.TrimSpace(coalesceText(item.LatestErrorCode)) != "" {
		return strings.ToUpper(strings.ReplaceAll(*item.LatestErrorCode, "-", "_"))
	}
	if strings.TrimSpace(coalesceText(job.LatestErrorCode)) != "" {
		return strings.ToUpper(strings.ReplaceAll(*job.LatestErrorCode, "-", "_"))
	}
	return strings.ToUpper(job.JobIntent) + "_FAILED"
}

func deriveSeverity(job jobSnapshot, category string) string {
	if job.JobFamily == "TRANSFER" {
		return SeverityCritical
	}
	switch category {
	case CategoryNodePermission, CategoryTransfer, CategoryCleanup:
		return SeverityCritical
	case CategoryVerify, CategoryConflict:
		return SeverityWarning
	default:
		return SeverityWarning
	}
}

func deriveIssueTitle(job jobSnapshot, item jobItemSnapshot, context issueLinkContext) string {
	if context.AssetLabel != nil {
		return *context.AssetLabel + " 处理失败"
	}
	if context.EndpointLabel != nil {
		return *context.EndpointLabel + " 处理失败"
	}
	if strings.TrimSpace(item.Title) != "" {
		return item.Title + " 执行失败"
	}
	return job.Title + " 执行失败"
}

func deriveObjectLabel(context issueLinkContext, item jobItemSnapshot, job jobSnapshot) string {
	if context.AssetLabel != nil && context.EndpointLabel != nil {
		return *context.AssetLabel + " / " + *context.EndpointLabel
	}
	if context.AssetLabel != nil {
		return *context.AssetLabel
	}
	if context.EndpointLabel != nil {
		return *context.EndpointLabel
	}
	if item.TargetPath != nil {
		return *item.TargetPath
	}
	if item.SourcePath != nil {
		return *item.SourcePath
	}
	if context.Path != nil {
		return *context.Path
	}
	return job.Title
}

func deriveSuggestion(context issueLinkContext) (*string, *string, *string) {
	if context.EntryID != nil {
		return ptr("建议回到文件中心查看目标条目和上下文，再决定是否重试。"), ptr("OPEN_FILE"), ptr("打开文件中心")
	}
	if context.EndpointID != nil {
		return ptr("建议先检查存储节点连接、挂载路径与可写状态，再决定是否重试。"), ptr("OPEN_STORAGE"), ptr("打开存储节点")
	}
	return ptr("建议先检查关联任务详情和错误上下文，再决定是否重试。"), ptr("RETRY"), ptr("重试")
}

func buildImpactSummary(context issueLinkContext, blocking bool) issuedto.ImpactSummary {
	return issuedto.ImpactSummary{
		AssetCount:          len(context.AssetIDs),
		ReplicaCount:        len(context.ReplicaIDs),
		DirectoryCount:      len(context.DirectoryIDs),
		EndpointCount:       max(len(context.EndpointIDs), len(context.StorageNodeIDs)),
		BlocksStatusCommit:  blocking,
		BlocksTaskExecution: blocking,
	}
}

func buildIssueLinks(job jobSnapshot, item *jobItemSnapshot, context issueLinkContext) []issueObjectLink {
	links := []issueObjectLink{
		{
			LinkRole:    "SOURCE_JOB",
			ObjectType:  "JOB",
			JobID:       ptr(job.ID),
			ObjectLabel: ptr(job.Title),
		},
	}
	if item != nil {
		links = append(links, issueObjectLink{
			LinkRole:    "SOURCE_JOB_ITEM",
			ObjectType:  "JOB_ITEM",
			JobItemID:   ptr(item.ID),
			ObjectLabel: ptr(item.Title),
		})
	}
	if context.AssetID != nil {
		links = append(links, issueObjectLink{
			LinkRole:    "AFFECTED_ASSET",
			ObjectType:  "ASSET",
			AssetID:     context.AssetID,
			ObjectLabel: context.AssetLabel,
		})
	}
	if context.DirectoryID != nil {
		links = append(links, issueObjectLink{
			LinkRole:    "AFFECTED_DIRECTORY",
			ObjectType:  "DIRECTORY",
			DirectoryID: context.DirectoryID,
			ObjectLabel: context.Path,
		})
	}
	if context.EndpointID != nil {
		links = append(links, issueObjectLink{
			LinkRole:    "AFFECTED_MOUNT",
			ObjectType:  "MOUNT",
			MountID:     context.EndpointID,
			ObjectLabel: context.EndpointLabel,
		})
	}
	for storageNodeID := range context.StorageNodeIDs {
		id := storageNodeID
		links = append(links, issueObjectLink{
			LinkRole:      "AFFECTED_STORAGE_NODE",
			ObjectType:    "STORAGE_NODE",
			StorageNodeID: &id,
		})
	}
	return links
}

func historyActionLabel(eventType string) string {
	switch eventType {
	case EventDetected:
		return "自动发现"
	case EventUpdated:
		return "更新"
	case EventRetry:
		return "重试"
	case EventConfirmed:
		return "标记已确认"
	case EventIgnored:
		return "忽略"
	case EventResolved:
		return "已解决"
	case EventArchived:
		return "归档"
	case EventAutoReopened:
		return "自动重开"
	default:
		return eventType
	}
}

func collectDedupeKeys(candidates []issueCandidate) []string {
	items := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, candidate.DedupeKey)
	}
	return items
}

func extractTaskID(snapshot []byte) string {
	if len(snapshot) == 0 {
		return ""
	}
	var source issuedto.SourceContext
	if err := json.Unmarshal(snapshot, &source); err != nil || source.TaskID == nil {
		return ""
	}
	return *source.TaskID
}

func formatOptionalRFC3339(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func marshalNullableJSON(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func buildCode(prefix string, now time.Time) string {
	token := make([]byte, 4)
	if _, err := rand.Read(token); err != nil {
		return fmt.Sprintf("%s-%d", prefix, now.UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", prefix, now.UnixNano(), hex.EncodeToString(token))
}

func ptr(value string) *string {
	return &value
}

func statusMaybePtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func isHistoricalStatus(status string) bool {
	return status == StatusIgnored || status == StatusResolved || status == StatusArchived
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func coalesceText(values ...*string) string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return *value
		}
	}
	return ""
}

func coalesceStringPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return value
		}
	}
	return nil
}

type issueRow struct {
	ID                   string
	Code                 string
	LibraryID            *string
	IssueCategory        string
	IssueType            string
	Nature               string
	SourceDomain         string
	Severity             string
	Status               string
	DedupeKey            *string
	Title                string
	Summary              string
	ObjectLabel          string
	AssetLabel           *string
	SuggestedAction      *string
	SuggestedActionLabel *string
	Suggestion           *string
	Detail               *string
	SourceSnapshot       []byte
	ImpactSnapshot       []byte
	FirstDetectedAt      time.Time
	LastDetectedAt       time.Time
	LastStatusChangedAt  time.Time
	ResolvedAt           *time.Time
	ArchivedAt           *time.Time
	LatestEventAt        *time.Time
	LatestErrorCode      *string
	LatestErrorMessage   *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type jobSnapshot struct {
	ID                 string
	LibraryID          *string
	JobFamily          string
	JobIntent          string
	SourceDomain       string
	Title              string
	LatestErrorCode    *string
	LatestErrorMessage *string
	Status             string
}

type jobItemSnapshot struct {
	ID                 string
	Title              string
	Status             string
	SourcePath         *string
	TargetPath         *string
	LatestErrorCode    *string
	LatestErrorMessage *string
}

type issueLinkRow struct {
	LinkRole       string
	ObjectType     string
	AssetID        *string
	AssetReplicaID *string
	DirectoryID    *string
	MountID        *string
	StorageNodeID  *string
}

type issueLinkContext struct {
	AssetID        *string
	DirectoryID    *string
	EntryID        *string
	EndpointID     *string
	EndpointLabel  *string
	AssetLabel     *string
	Path           *string
	SourceLabel    *string
	RouteLabel     *string
	AssetIDs       map[string]struct{}
	ReplicaIDs     map[string]struct{}
	DirectoryIDs   map[string]struct{}
	EndpointIDs    map[string]struct{}
	StorageNodeIDs map[string]struct{}
}

type issueCandidate struct {
	LibraryID            *string
	IssueCategory        string
	IssueType            string
	Nature               string
	SourceDomain         string
	Severity             string
	DedupeKey            string
	Title                string
	Summary              string
	ObjectLabel          string
	AssetLabel           *string
	SuggestedAction      *string
	SuggestedActionLabel *string
	Suggestion           *string
	Detail               *string
	SourceSnapshot       issuedto.SourceContext
	ImpactSnapshot       issuedto.ImpactSummary
	LatestErrorCode      *string
	LatestErrorMessage   *string
	Links                []issueObjectLink
}

type issueObjectLink struct {
	LinkRole        string
	ObjectType      string
	JobID           *string
	JobItemID       *string
	AssetID         *string
	AssetReplicaID  *string
	DirectoryID     *string
	MountID         *string
	StorageNodeID   *string
	ExternalRefType *string
	ExternalRefID   *string
	ObjectLabel     *string
}

type issueEventInput struct {
	EventType     string
	ActionKey     *string
	FromStatus    *string
	ToStatus      *string
	ActorType     string
	ActorRefID    *string
	OperatorLabel *string
	Message       *string
	Payload       map[string]any
	CreatedAt     time.Time
}
