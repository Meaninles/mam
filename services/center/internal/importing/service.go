package importing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/assets"
	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/jobs"
	importdto "mare/shared/contracts/dto/importing"
	jobdto "mare/shared/contracts/dto/job"
)

const (
	sessionScanPending = "PENDING"
	sessionScanReady   = "READY"
	sessionScanFailed  = "FAILED"

	sessionStateReady        = "READY"
	sessionStateImporting    = "IMPORTING"
	sessionStatePartial      = "PARTIAL_SUCCESS"
	sessionStateIssue        = "ISSUE"
	sessionStateDisconnected = "DISCONNECTED"

	planStatusDraft     = "DRAFT"
	planStatusReady     = "READY"
	planStatusSubmitted = "SUBMITTED"
	planStatusImporting = "IMPORTING"
	planStatusCompleted = "COMPLETED"
	planStatusFailed    = "FAILED"

	entryStatusPending   = "PENDING"
	entryStatusQueued    = "QUEUED"
	entryStatusCompleted = "COMPLETED"
	entryStatusFailed    = "FAILED"
	entryStatusConflict  = "CONFLICT"
	entryStatusSkipped   = "SKIPPED"

	reportStatusQueued    = "QUEUED"
	reportStatusRunning   = "RUNNING"
	reportStatusPartial   = "PARTIAL_SUCCESS"
	reportStatusFailed    = "FAILED"
	reportStatusCompleted = "COMPLETED"

	defaultTargetStrategy     = "按资产库推荐目标端"
	defaultDestinationRootDir = "/"
)

type AgentBridge interface {
	DiscoverSources(ctx context.Context, callbackBaseURL string) ([]importdto.SourceDescriptor, error)
	BrowseSource(ctx context.Context, callbackBaseURL string, request importdto.BrowseRequest) (importdto.BrowseResponse, error)
	ExecuteImport(ctx context.Context, callbackBaseURL string, request importdto.ExecuteImportRequest) (importdto.ExecuteImportResponse, error)
}

type JobCreator interface {
	CreateJob(ctx context.Context, input jobs.CreateJobInput) (jobdto.CreateResponse, error)
	LoadJobDetail(ctx context.Context, id string) (jobdto.Detail, error)
}

type AssetService interface {
	IngestImportedReplica(ctx context.Context, input assets.ImportedReplicaInput) error
}

type Service struct {
	pool       *pgxpool.Pool
	bridge     AgentBridge
	jobCreator JobCreator
	assets     AssetService
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, bridge AgentBridge, jobCreator JobCreator, assetService AssetService) *Service {
	return &Service{
		pool:       pool,
		bridge:     bridge,
		jobCreator: jobCreator,
		assets:     assetService,
		now:        time.Now,
	}
}

type agentRow struct {
	AgentID         string
	CallbackBaseURL string
}

type sessionRow struct {
	ID               string
	AgentID          string
	DeviceKey        string
	DeviceLabel      string
	DeviceType       string
	SourcePath       string
	MountPath        string
	ScanStatus       string
	SessionStatus    string
	LastErrorCode    *string
	LastErrorMessage *string
	ConnectedAt      time.Time
	LastSeenAt       time.Time
	DisconnectedAt   *time.Time
	CapacityBytes    *int64
	AvailableBytes   *int64
	CallbackBaseURL  string
}

type planRow struct {
	ID                  string
	SessionID           string
	LibraryID           *string
	Status              string
	TargetStrategy      string
	DestinationRootPath string
	HasBlockingIssues   bool
	LastPrecheckedAt    *time.Time
	PrecheckSummary     []byte
	SubmittedAt         *time.Time
	FinishedAt          *time.Time
	UpdatedAt           time.Time
}

type entryRow struct {
	ID             string
	SessionID      string
	Name           string
	RelativePath   string
	FileKind       string
	SizeBytes      int64
	ModifiedAt     time.Time
	TargetMountIDs []string
	ImportStatus   string
}

type targetEndpointRow struct {
	ID               string
	EndpointID       string
	LibraryID        string
	Label            string
	NodeType         string
	SourcePath       string
	RelativeRootPath string
	AvailableBytes   *int64
}

type selectionRow struct {
	ID             string
	PlanID         string
	SessionID      string
	EntryType      string
	RelativePath   string
	Name           string
	TargetMountIDs []string
}

func (s *Service) RefreshDashboard(ctx context.Context) (importdto.DashboardResponse, error) {
	if err := s.refreshSessions(ctx); err != nil && !errorsAsNotFound(err) {
		return importdto.DashboardResponse{}, err
	}
	return s.LoadDashboard(ctx)
}

func (s *Service) LoadDashboard(ctx context.Context) (importdto.DashboardResponse, error) {
	libraries, err := s.loadLibraries(ctx)
	if err != nil {
		return importdto.DashboardResponse{}, err
	}
	targets, err := s.loadTargetEndpoints(ctx)
	if err != nil {
		return importdto.DashboardResponse{}, err
	}
	drafts, err := s.loadDrafts(ctx)
	if err != nil {
		return importdto.DashboardResponse{}, err
	}
	targetsByLibrary := make(map[string][]string)
	for _, target := range targets {
		targetsByLibrary[target.LibraryID] = append(targetsByLibrary[target.LibraryID], target.ID)
	}
	devices, err := s.loadDevices(ctx, drafts, targetsByLibrary, targets)
	if err != nil {
		return importdto.DashboardResponse{}, err
	}
	reports, err := s.loadReports(ctx)
	if err != nil {
		return importdto.DashboardResponse{}, err
	}
	return importdto.DashboardResponse{
		Libraries:       libraries,
		Devices:         devices,
		Drafts:          drafts,
		Reports:         reports,
		TargetEndpoints: targets,
	}, nil
}

func (s *Service) SetDraftLibrary(ctx context.Context, draftID string, libraryID string) (importdto.MutationResponse, error) {
	if strings.TrimSpace(libraryID) == "" {
		return importdto.MutationResponse{}, apperrors.BadRequest("资产库不能为空")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	var sessionID string
	if err := tx.QueryRow(ctx, `SELECT session_id FROM import_plans WHERE id = $1`, draftID).Scan(&sessionID); err != nil {
		return importdto.MutationResponse{}, apperrors.NotFound("导入草稿不存在")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE import_plans
		SET library_id = $2,
		    status = $3,
		    updated_at = $4
		WHERE id = $1
	`, draftID, libraryID, planStatusDraft, s.now().UTC()); err != nil {
		return importdto.MutationResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE import_session_entries
		SET target_mount_ids = '{}'::text[],
		    updated_at = $2
		WHERE session_id = $1
	`, sessionID, s.now().UTC()); err != nil {
		return importdto.MutationResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return importdto.MutationResponse{}, err
	}
	return importdto.MutationResponse{Message: "已切换导入资产库"}, nil
}

func (s *Service) ApplyTargetToAll(ctx context.Context, sessionID string, targetID string) (importdto.MutationResponse, error) {
	plan, err := s.loadPlanBySession(ctx, sessionID)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	if plan.LibraryID == nil {
		return importdto.MutationResponse{}, apperrors.BadRequest("请先选择资产库")
	}
	target, err := s.loadTargetByID(ctx, targetID)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	if target.LibraryID != *plan.LibraryID {
		return importdto.MutationResponse{}, apperrors.BadRequest("目标端不属于当前资产库")
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE import_session_entries
		SET target_mount_ids = CASE
			WHEN $2 = ANY(target_mount_ids) THEN target_mount_ids
			ELSE array_append(target_mount_ids, $2)
		END,
		    updated_at = $3
		WHERE session_id = $1
		  AND entry_type = 'FILE'
		  AND import_status <> 'SKIPPED'
	`, sessionID, targetID, s.now().UTC()); err != nil {
		return importdto.MutationResponse{}, err
	}
	return importdto.MutationResponse{Message: "已将目标端应用到当前设备全部可导入文件"}, nil
}

func (s *Service) RemoveTargetFromAll(ctx context.Context, sessionID string, targetID string) (importdto.MutationResponse, error) {
	if _, err := s.pool.Exec(ctx, `
		UPDATE import_session_entries
		SET target_mount_ids = array_remove(target_mount_ids, $2),
		    updated_at = $3
		WHERE session_id = $1
		  AND entry_type = 'FILE'
	`, sessionID, targetID, s.now().UTC()); err != nil {
		return importdto.MutationResponse{}, err
	}
	return importdto.MutationResponse{Message: "已取消当前设备全部文件在该目标端的导入配置"}, nil
}

func (s *Service) UpdateSourceTargets(ctx context.Context, entryID string, targetIDs []string) (importdto.MutationResponse, error) {
	return importdto.MutationResponse{}, apperrors.BadRequest("当前接口已弃用")
}

func (s *Service) SaveSelectionTargets(
	ctx context.Context,
	sessionID string,
	entryType string,
	name string,
	relativePath string,
	targetIDs []string,
) (importdto.MutationResponse, error) {
	plan, err := s.loadPlanBySession(ctx, sessionID)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	if plan.LibraryID == nil {
		return importdto.MutationResponse{}, apperrors.BadRequest("请先选择资产库")
	}
	entryType = strings.ToUpper(strings.TrimSpace(entryType))
	if entryType != "FILE" && entryType != "DIRECTORY" {
		return importdto.MutationResponse{}, apperrors.BadRequest("导入对象类型无效")
	}
	relativePath = strings.Trim(strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/"), "/")
	if relativePath == "" {
		return importdto.MutationResponse{}, apperrors.BadRequest("导入对象路径不能为空")
	}
	for _, targetID := range targetIDs {
		target, err := s.loadTargetByID(ctx, targetID)
		if err != nil {
			return importdto.MutationResponse{}, err
		}
		if target.LibraryID != *plan.LibraryID {
			return importdto.MutationResponse{}, apperrors.BadRequest("所选目标端必须属于同一资产库")
		}
	}
	now := s.now().UTC()
	if len(targetIDs) == 0 {
		if _, err := s.pool.Exec(ctx, `
			DELETE FROM import_plan_items
			WHERE plan_id = $1 AND relative_path = $2
		`, plan.ID, relativePath); err != nil {
			return importdto.MutationResponse{}, err
		}
		return importdto.MutationResponse{Message: "已取消当前对象的导入目标端"}, nil
	}
	selectionID := buildCode("import-plan-item", now)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO import_plan_items (
			id, plan_id, session_id, entry_type, relative_path, name, target_mount_ids, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		ON CONFLICT (plan_id, relative_path) DO UPDATE SET
			entry_type = EXCLUDED.entry_type,
			name = EXCLUDED.name,
			target_mount_ids = EXCLUDED.target_mount_ids,
			updated_at = EXCLUDED.updated_at
	`, selectionID, plan.ID, sessionID, entryType, relativePath, strings.TrimSpace(name), targetIDs, now); err != nil {
		return importdto.MutationResponse{}, err
	}
	return importdto.MutationResponse{Message: "已更新当前对象的导入目标端"}, nil
}

func (s *Service) BrowseSession(ctx context.Context, sessionID string, path string, limit int, offset int) (importdto.BrowseSessionResponse, error) {
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return importdto.BrowseSessionResponse{}, err
	}
	normalizedPath := strings.Trim(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"), "/")
	response, err := s.bridge.BrowseSource(ctx, session.CallbackBaseURL, importdto.BrowseRequest{
		SourcePath:   session.SourcePath,
		RelativePath: optionalPath(normalizedPath),
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		return importdto.BrowseSessionResponse{}, err
	}
	selections, err := s.loadSelectionsBySession(ctx, sessionID)
	if err != nil {
		return importdto.BrowseSessionResponse{}, err
	}
	items := make([]importdto.BrowserNodeRecord, 0, len(response.Entries))
	for _, entry := range response.Entries {
		size := (*string)(nil)
		if entry.SizeBytes != nil {
			formatted := formatBytes(*entry.SizeBytes)
			size = &formatted
		}
		selectedTargets := []string{}
		if selection, ok := selections[entry.RelativePath]; ok {
			selectedTargets = nonNilStrings(selection.TargetMountIDs)
		}
		items = append(items, importdto.BrowserNodeRecord{
			ID:                "browser:" + sessionID + ":" + entry.RelativePath,
			DeviceSessionID:   sessionID,
			EntryType:         entry.EntryType,
			Name:              entry.Name,
			RelativePath:      entry.RelativePath,
			FileKind:          entry.FileKind,
			Size:              size,
			ModifiedAt:        entry.ModifiedAt,
			IsHidden:          entry.IsHidden,
			HasChildren:       entry.HasChildren,
			TargetEndpointIDs: selectedTargets,
		})
	}
	currentPath := "/"
	if normalizedPath != "" {
		currentPath = "/" + normalizedPath
	}
	return importdto.BrowseSessionResponse{
		SessionID:   sessionID,
		CurrentPath: currentPath,
		Items:       items,
		Total:       response.Total,
		Limit:       response.Limit,
		Offset:      response.Offset,
		HasMore:     response.HasMore,
	}, nil
}

func (s *Service) RefreshPrecheck(ctx context.Context, draftID string) (importdto.MutationResponse, error) {
	plan, err := s.loadPlan(ctx, draftID)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	session, err := s.loadSession(ctx, plan.SessionID)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	selections, err := s.loadSelectionsByPlan(ctx, draftID)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	summary, status, err := s.buildSelectionPrecheck(ctx, session, plan, selections)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	now := s.now().UTC()
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return importdto.MutationResponse{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE import_plans
		SET precheck_summary = $2,
		    has_blocking_issues = $3,
		    selected_file_count = $4,
		    total_size_bytes = $5,
		    status = $6,
		    last_prechecked_at = $7,
		    updated_at = $7
		WHERE id = $1
	`, draftID, summaryJSON, summary.BlockingCount > 0, len(selections), 0, status, now); err != nil {
		return importdto.MutationResponse{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE import_sessions
		SET session_status = $2,
		    updated_at = $3
		WHERE id = $1
	`, session.ID, resolveSessionStateFromPrecheck(summary), now); err != nil {
		return importdto.MutationResponse{}, err
	}
	return importdto.MutationResponse{Message: "预检结果已刷新"}, nil
}

func (s *Service) Submit(ctx context.Context, sessionID string) (importdto.SubmitResponse, error) {
	plan, err := s.loadPlanBySession(ctx, sessionID)
	if err != nil {
		return importdto.SubmitResponse{}, err
	}
	if plan.LibraryID == nil {
		return importdto.SubmitResponse{}, apperrors.BadRequest("请先选择资产库")
	}
	if _, err := s.RefreshPrecheck(ctx, plan.ID); err != nil {
		return importdto.SubmitResponse{}, err
	}
	plan, err = s.loadPlan(ctx, plan.ID)
	if err != nil {
		return importdto.SubmitResponse{}, err
	}
	if plan.HasBlockingIssues {
		return importdto.SubmitResponse{}, apperrors.BadRequest("仍存在阻塞项，请先处理后再提交导入")
	}
	selections, err := s.loadSelectionsByPlan(ctx, plan.ID)
	if err != nil {
		return importdto.SubmitResponse{}, err
	}
	targetLabels := make([]string, 0)
	targetSeen := make(map[string]struct{})
	for _, selection := range selections {
		if len(selection.TargetMountIDs) == 0 {
			continue
		}
		for _, targetID := range selection.TargetMountIDs {
			if _, ok := targetSeen[targetID]; ok {
				continue
			}
			target, err := s.loadTargetByID(ctx, targetID)
			if err != nil {
				return importdto.SubmitResponse{}, err
			}
			targetSeen[targetID] = struct{}{}
			targetLabels = append(targetLabels, target.Label)
		}
	}
	if len(selections) == 0 {
		return importdto.SubmitResponse{}, apperrors.BadRequest("当前没有可提交的导入文件")
	}

	now := s.now().UTC()
	reportID := buildCode("import-report", now)
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return importdto.SubmitResponse{}, err
	}

	items := make([]jobs.CreateItemInput, 0, len(selections))
	for _, selection := range selections {
		sourceAbsolutePath := filepath.Join(session.SourcePath, filepath.FromSlash(selection.RelativePath))
		targetPath := joinLogicalPath(plan.DestinationRootPath, selection.RelativePath)
		itemLinks := make([]jobs.CreateObjectLinkInput, 0, len(selection.TargetMountIDs))
		for _, targetID := range selection.TargetMountIDs {
			targetIDCopy := targetID
			itemLinks = append(itemLinks, jobs.CreateObjectLinkInput{
				LinkRole:   jobs.LinkRoleTargetMount,
				ObjectType: jobs.ObjectTypeMount,
				MountID:    &targetIDCopy,
			})
		}
		items = append(items, jobs.CreateItemInput{
			ItemKey:    selection.ID,
			ItemType:   jobs.ItemTypeAssetReplicaTransfer,
			RouteType:  ptr("COPY"),
			Title:      fmt.Sprintf("导入%s：%s", mapSelectionTitleType(selection.EntryType), selection.Name),
			Summary:    fmt.Sprintf("导入到 %d 个目标端", len(selection.TargetMountIDs)),
			SourcePath: &sourceAbsolutePath,
			TargetPath: &targetPath,
			Links:      itemLinks,
		})
	}

	jobResult, err := s.jobCreator.CreateJob(ctx, jobs.CreateJobInput{
		LibraryID:     plan.LibraryID,
		JobFamily:     jobs.JobFamilyTransfer,
		JobIntent:     jobs.JobIntentImport,
		RouteType:     ptr("COPY"),
		Title:         fmt.Sprintf("%s 导入", session.DeviceLabel),
		Summary:       fmt.Sprintf("从 %s 导入到 %s", session.MountPath, strings.Join(targetLabels, "、")),
		SourceDomain:  jobs.SourceDomainImportCenter,
		SourceRefID:   &session.ID,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		SourceSnapshot: map[string]any{
			"sessionId":    session.ID,
			"planId":       plan.ID,
			"reportId":     reportID,
			"deviceLabel":  session.DeviceLabel,
			"mountPath":    session.MountPath,
			"libraryId":    *plan.LibraryID,
			"targetLabels": targetLabels,
		},
		Items: items,
	})
	if err != nil {
		return importdto.SubmitResponse{}, err
	}

	targetSummaries := make([]importdto.ReportTargetSummary, 0, len(targetSeen))
	for targetID := range targetSeen {
		target, err := s.loadTargetByID(ctx, targetID)
		if err != nil {
			return importdto.SubmitResponse{}, err
		}
		targetSummaries = append(targetSummaries, importdto.ReportTargetSummary{
			EndpointID:      target.ID,
			Label:           target.Label,
			Status:          "等待执行",
			SuccessCount:    0,
			FailedCount:     0,
			TransferredSize: "0 B",
		})
	}
	targetSummariesJSON, err := json.Marshal(targetSummaries)
	if err != nil {
		return importdto.SubmitResponse{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return importdto.SubmitResponse{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO import_reports (
			id, session_id, plan_id, job_id, library_id, status, title, verify_summary, target_summaries,
			issue_ids, success_count, failed_count, partial_count, file_count,
			submitted_at, latest_updated_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			'{}'::text[], 0, 0, 0, $10,
			$11, $11, $11, $11
		)
	`, reportID, session.ID, plan.ID, jobResult.JobID, plan.LibraryID, reportStatusQueued,
		fmt.Sprintf("%s / 刚提交的导入作业", session.DeviceLabel),
		"导入作业已创建，等待本地执行器执行。",
		targetSummariesJSON, len(selections), now); err != nil {
		return importdto.SubmitResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE import_plans
		SET status = $2,
		    submitted_at = $3,
		    updated_at = $3
		WHERE id = $1
	`, plan.ID, planStatusSubmitted, now); err != nil {
		return importdto.SubmitResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE import_sessions
		SET session_status = $2,
		    updated_at = $3
		WHERE id = $1
	`, session.ID, sessionStateImporting, now); err != nil {
		return importdto.SubmitResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM import_session_entries WHERE session_id = $1
	`, session.ID); err != nil {
		return importdto.SubmitResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return importdto.SubmitResponse{}, err
	}

	report, err := s.loadReport(ctx, reportID)
	if err != nil {
		return importdto.SubmitResponse{}, err
	}
	return importdto.SubmitResponse{
		Message:  "已提交导入作业，任务已加入队列",
		ReportID: reportID,
		Report:   report,
	}, nil
}

func (s *Service) ExecuteImportJobItem(ctx context.Context, execution jobs.ExecutionContext) error {
	sourceSnapshot, ok := execution.Job.SourceSnapshot.(map[string]any)
	if !ok {
		return fmt.Errorf("导入作业缺少来源快照")
	}
	reportID := stringValue(sourceSnapshot["reportId"])
	sessionID := stringValue(sourceSnapshot["sessionId"])
	planID := stringValue(sourceSnapshot["planId"])
	libraryID := stringValue(sourceSnapshot["libraryId"])
	if reportID == "" || sessionID == "" || planID == "" || libraryID == "" {
		return fmt.Errorf("导入作业来源快照不完整")
	}

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return err
	}
	selection, err := s.loadSelection(ctx, execution.Item.ItemKey)
	if err != nil {
		return err
	}
	plan, err := s.loadPlan(ctx, planID)
	if err != nil {
		return err
	}
	targets, err := s.loadTargetsByIDs(ctx, selection.TargetMountIDs)
	if err != nil {
		return s.failEntry(ctx, reportID, session.ID, selection.ID, err)
	}
	targetByID := make(map[string]targetEndpointRow, len(targets))
	requestTargets := make([]importdto.ExecuteImportTarget, 0, len(targets))
	for _, target := range targets {
		targetByID[target.ID] = target
		requestTargets = append(requestTargets, importdto.ExecuteImportTarget{TargetID: target.ID, NodeType: target.NodeType, PreserveMtime: true})
	}
	filesToImport := []importFileCandidate{}
	if selection.EntryType == "DIRECTORY" {
		filesToImport, err = s.expandDirectorySelection(ctx, session, selection.RelativePath)
	} else {
		filesToImport = []importFileCandidate{{
			RelativePath: selection.RelativePath,
			PhysicalPath: filepath.Join(session.SourcePath, filepath.FromSlash(selection.RelativePath)),
		}}
	}
	if err != nil {
		return s.failEntry(ctx, reportID, session.ID, selection.ID, err)
	}
	for _, candidate := range filesToImport {
		logicalPath := joinLogicalPath(plan.DestinationRootPath, candidate.RelativePath)
		targetRequest := make([]importdto.ExecuteImportTarget, 0, len(targets))
		for _, target := range targets {
			targetRequest = append(targetRequest, importdto.ExecuteImportTarget{
				TargetID:      target.ID,
				NodeType:      target.NodeType,
				PhysicalPath:  resolvePhysicalImportPath(target.SourcePath, logicalPath),
				PreserveMtime: true,
			})
		}
		result, execErr := s.bridge.ExecuteImport(ctx, session.CallbackBaseURL, importdto.ExecuteImportRequest{
			SourcePath: candidate.PhysicalPath,
			Targets:    targetRequest,
		})
		if execErr != nil {
			return s.failEntry(ctx, reportID, session.ID, selection.ID, execErr)
		}
		for _, targetResult := range result.Targets {
			target, ok := targetByID[targetResult.TargetID]
			if !ok {
				continue
			}
			modifiedAt, parseErr := time.Parse(time.RFC3339, targetResult.ModifiedAt)
			if parseErr != nil {
				modifiedAt = s.now().UTC()
			}
			if err := s.assets.IngestImportedReplica(ctx, assets.ImportedReplicaInput{
				LibraryID:    libraryID,
				MountID:      target.ID,
				LogicalPath:  logicalPath,
				PhysicalPath: targetResult.PhysicalPath,
				SizeBytes:    targetResult.BytesWritten,
				ModifiedAt:   modifiedAt,
				FileKind:     classifyFileKindFromPath(candidate.RelativePath),
			}); err != nil {
				return s.failEntry(ctx, reportID, session.ID, selection.ID, err)
			}
		}
	}
	if err := s.completeEntry(ctx, reportID, session.ID, selection.ID); err != nil {
		return err
	}
	return nil
}

func (s *Service) completeEntry(ctx context.Context, reportID string, sessionID string, entryID string) error {
	return s.refreshReportAggregate(ctx, reportID, sessionID)
}

func (s *Service) failEntry(ctx context.Context, reportID string, sessionID string, entryID string, execErr error) error {
	if err := s.refreshReportAggregate(ctx, reportID, sessionID); err != nil {
		return err
	}
	return execErr
}

func (s *Service) refreshReportAggregate(ctx context.Context, reportID string, sessionID string) error {
	var jobID string
	if err := s.pool.QueryRow(ctx, `SELECT job_id FROM import_reports WHERE id = $1`, reportID).Scan(&jobID); err != nil {
		return err
	}
	detail, err := s.jobCreator.LoadJobDetail(ctx, jobID)
	if err != nil {
		return err
	}
	total := len(detail.Items)
	success := 0
	failed := 0
	for _, item := range detail.Items {
		if item.Status == jobs.ItemStatusCompleted {
			success++
		}
		if item.Status == jobs.ItemStatusFailed || item.Status == jobs.ItemStatusCanceled {
			failed++
		}
	}
	status := reportStatusRunning
	verifySummary := "导入执行中"
	if success+failed >= total {
		switch {
		case success == total:
			status = reportStatusCompleted
			verifySummary = "导入已完成"
		case success > 0:
			status = reportStatusPartial
			verifySummary = "导入部分成功，请处理失败项"
		default:
			status = reportStatusFailed
			verifySummary = "导入失败，请查看异常和任务详情"
		}
	}
	now := s.now().UTC()
	if _, err := s.pool.Exec(ctx, `
		UPDATE import_reports
		SET status = $2,
		    success_count = $3,
		    failed_count = $4,
		    partial_count = CASE WHEN $4 > 0 AND $3 > 0 THEN $4 ELSE 0 END,
		    verify_summary = $5,
		    latest_updated_at = $6,
		    finished_at = CASE WHEN $2 IN ('COMPLETED', 'FAILED', 'PARTIAL_SUCCESS') THEN COALESCE(finished_at, $6) ELSE NULL END,
		    updated_at = $6
		WHERE id = $1
	`, reportID, status, success, failed, verifySummary, now); err != nil {
		return err
	}
	sessionState := sessionStateImporting
	planState := planStatusImporting
	if status == reportStatusCompleted {
		sessionState = sessionStateReady
		planState = planStatusCompleted
	}
	if status == reportStatusPartial {
		sessionState = sessionStatePartial
		planState = planStatusFailed
	}
	if status == reportStatusFailed {
		sessionState = sessionStateIssue
		planState = planStatusFailed
	}
	if _, err := s.pool.Exec(ctx, `UPDATE import_sessions SET session_status = $2, updated_at = $3 WHERE id = $1`, sessionID, sessionState, now); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE import_plans
		SET status = $2,
		    finished_at = CASE WHEN $2 IN ('COMPLETED', 'FAILED') THEN COALESCE(finished_at, $3) ELSE NULL END,
		    updated_at = $3
		WHERE session_id = $1
	`, sessionID, planState, now)
	return err
}

func (s *Service) refreshSessions(ctx context.Context) error {
	agent, err := s.loadActiveAgent(ctx)
	if err != nil {
		return err
	}
	sources, err := s.bridge.DiscoverSources(ctx, agent.CallbackBaseURL)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		sessionID, err := s.upsertSession(ctx, agent.AgentID, source, now)
		if err != nil {
			return err
		}
		seen[source.DeviceKey] = struct{}{}
		if _, err := s.pool.Exec(ctx, `
			UPDATE import_sessions
			SET scan_status = $2,
			    session_status = CASE WHEN session_status = 'DISCONNECTED' THEN $3 ELSE session_status END,
			    last_error_code = NULL,
			    last_error_message = NULL,
			    updated_at = $4,
			    last_seen_at = $4,
			    disconnected_at = NULL
			WHERE id = $1
		`, sessionID, sessionScanReady, sessionStateReady, now); err != nil {
			return err
		}
		if err := s.ensurePlan(ctx, sessionID, now); err != nil {
			return err
		}
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE import_sessions
		SET session_status = $2,
		    disconnected_at = $3,
		    updated_at = $3
		WHERE agent_id = $1
		  AND device_key <> ALL($4)
		  AND disconnected_at IS NULL
	`, agent.AgentID, sessionStateDisconnected, now, keysOf(seen)); err != nil {
		return err
	}
	return nil
}

func (s *Service) upsertSession(ctx context.Context, agentID string, source importdto.SourceDescriptor, now time.Time) (string, error) {
	var sessionID string
	err := s.pool.QueryRow(ctx, `
		SELECT id
		FROM import_sessions
		WHERE agent_id = $1 AND device_key = $2
	`, agentID, source.DeviceKey).Scan(&sessionID)
	if err == pgx.ErrNoRows {
		sessionID = buildCode("import-session", now)
	} else if err != nil {
		return "", err
	}
	sourcePayload := source.SourceSnapshot
	if len(sourcePayload) == 0 {
		sourcePayload = map[string]any{}
	}
	sourceSnapshot, err := json.Marshal(sourcePayload)
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO import_sessions (
			id, agent_id, device_key, source_type, device_label, device_type, source_path, mount_path,
			volume_name, file_system, capacity_bytes, available_bytes,
			scan_status, session_status, source_snapshot, connected_at, last_seen_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $18
		)
		ON CONFLICT (agent_id, device_key) DO UPDATE SET
			device_label = EXCLUDED.device_label,
			device_type = EXCLUDED.device_type,
			source_path = EXCLUDED.source_path,
			mount_path = EXCLUDED.mount_path,
			volume_name = EXCLUDED.volume_name,
			file_system = EXCLUDED.file_system,
			capacity_bytes = EXCLUDED.capacity_bytes,
			available_bytes = EXCLUDED.available_bytes,
			scan_status = EXCLUDED.scan_status,
			session_status = EXCLUDED.session_status,
			source_snapshot = EXCLUDED.source_snapshot,
			last_seen_at = EXCLUDED.last_seen_at,
			disconnected_at = NULL,
			updated_at = EXCLUDED.updated_at
	`, sessionID, agentID, source.DeviceKey, source.SourceType, source.DeviceLabel, source.DeviceType,
		source.SourcePath, source.MountPath, source.VolumeName, source.FileSystem, source.CapacityBytes, source.AvailableBytes,
		sessionScanPending, sessionStateReady, sourceSnapshot, parseRFC3339OrNow(source.ConnectedAt, now), parseRFC3339OrNow(source.LastSeenAt, now), now)
	return sessionID, err
}

func (s *Service) replaceSessionEntries(ctx context.Context, sessionID string, browse importdto.BrowseResponse, now time.Time) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM import_session_entries WHERE session_id = $1`, sessionID); err != nil {
		return err
	}
	for _, entry := range browse.Entries {
		entryType := strings.TrimSpace(entry.EntryType)
		if entryType == "" {
			continue
		}
		id := buildCode("import-entry", now)
		modifiedAt := parseRFC3339OrNow(entry.ModifiedAt, now)
		var extension *string
		if entry.Extension != nil && strings.TrimSpace(*entry.Extension) != "" {
			ext := strings.TrimSpace(*entry.Extension)
			extension = &ext
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO import_session_entries (
				id, session_id, entry_type, relative_path, parent_relative_path, name, extension, file_kind,
				size_bytes, modified_at, import_status, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8,
				$9, $10, $11, $12, $12
			)
		`, id, sessionID, entryType, entry.RelativePath, parentLogicalPathOrNil(entry.RelativePath), entry.Name, extension, entry.FileKind,
			entry.SizeBytes, modifiedAt, entryStatusPending, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) ensurePlan(ctx context.Context, sessionID string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO import_plans (
			id, session_id, status, target_strategy, destination_root_path, created_at, updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $6
		WHERE NOT EXISTS (SELECT 1 FROM import_plans WHERE session_id = $2)
	`, buildCode("import-plan", now), sessionID, planStatusDraft, defaultTargetStrategy, defaultDestinationRootDir, now)
	return err
}

func (s *Service) buildPrecheck(ctx context.Context, session sessionRow, plan planRow, files []entryRow) (importdto.PrecheckSummary, string, error) {
	now := s.now().UTC().Format(time.RFC3339)
	summary := importdto.PrecheckSummary{
		UpdatedAt: now,
		Items:     make([]importdto.PrecheckItem, 0),
	}
	applyCheck := func(field *string, status string) {
		*field = status
		switch status {
		case "blocking":
			summary.BlockingCount++
		case "risk":
			summary.RiskCount++
		default:
			summary.PassedCount++
		}
	}

	sourceReadable := "passed"
	if session.ScanStatus == sessionScanFailed {
		sourceReadable = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{
			ID:     "source-readable",
			Label:  "来源目录可读",
			Status: "blocking",
			Detail: valueOrDefault(session.LastErrorMessage, "来源目录读取失败"),
		})
	}
	deviceOnline := "passed"
	if session.SessionStatus == sessionStateDisconnected {
		deviceOnline = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{
			ID:     "device-online",
			Label:  "来源设备在线",
			Status: "blocking",
			Detail: "来源设备已断开，请重新连接后再继续。",
		})
	}
	executorReady := "passed"
	if strings.TrimSpace(session.CallbackBaseURL) == "" {
		executorReady = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{
			ID:     "executor-ready",
			Label:  "本地执行器可用",
			Status: "blocking",
			Detail: "本地执行器未就绪，无法执行导入。",
		})
	}

	targetWritable := "passed"
	capacityReady := "passed"
	pathConflict := "passed"
	if plan.LibraryID == nil {
		targetWritable = "blocking"
		capacityReady = "blocking"
		pathConflict = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{
			ID:     "library-required",
			Label:  "已选择资产库",
			Status: "blocking",
			Detail: "请先选择导入目标资产库。",
		})
	} else {
		targets, err := s.loadTargetsForLibrary(ctx, *plan.LibraryID)
		if err != nil {
			return importdto.PrecheckSummary{}, "", err
		}
		targetMap := make(map[string]targetEndpointRow, len(targets))
		for _, target := range targets {
			targetMap[target.ID] = target
		}
		capacityByTarget := make(map[string]int64)
		for _, file := range files {
			if len(file.TargetMountIDs) == 0 {
				targetWritable = "blocking"
				summary.Items = append(summary.Items, importdto.PrecheckItem{
					ID:     "target-missing-" + file.ID,
					Label:  "文件目标端配置完整",
					Status: "blocking",
					Detail: fmt.Sprintf("文件“%s”还没有选择目标端。", file.Name),
				})
				continue
			}
			for _, targetID := range file.TargetMountIDs {
				target, ok := targetMap[targetID]
				if !ok {
					targetWritable = "blocking"
					summary.Items = append(summary.Items, importdto.PrecheckItem{
						ID:     "target-invalid-" + file.ID,
						Label:  "目标端可写",
						Status: "blocking",
						Detail: fmt.Sprintf("目标端 %s 不属于当前资产库或当前不可用。", targetID),
					})
					continue
				}
				capacityByTarget[target.ID] += file.SizeBytes
				conflict, err := s.detectPathConflict(ctx, *plan.LibraryID, joinLogicalPath(plan.DestinationRootPath, file.RelativePath))
				if err != nil {
					return importdto.PrecheckSummary{}, "", err
				}
				if conflict {
					pathConflict = "blocking"
					summary.Items = append(summary.Items, importdto.PrecheckItem{
						ID:     "path-conflict-" + file.ID,
						Label:  "目标路径冲突",
						Status: "blocking",
						Detail: fmt.Sprintf("文件“%s”将在资产库中产生同路径冲突。", file.Name),
					})
				}
			}
		}
		for _, target := range targets {
			if target.AvailableBytes == nil {
				continue
			}
			if capacityByTarget[target.ID] > *target.AvailableBytes {
				capacityReady = "blocking"
				summary.Items = append(summary.Items, importdto.PrecheckItem{
					ID:     "capacity-" + target.ID,
					Label:  "目标端容量充足",
					Status: "blocking",
					Detail: fmt.Sprintf("目标端“%s”剩余空间不足。", target.Label),
				})
			}
		}
	}

	applyCheck(&summary.Checks.SourceReadable, sourceReadable)
	applyCheck(&summary.Checks.TargetWritable, targetWritable)
	applyCheck(&summary.Checks.CapacityReady, capacityReady)
	applyCheck(&summary.Checks.PathConflict, pathConflict)
	applyCheck(&summary.Checks.DeviceOnline, deviceOnline)
	applyCheck(&summary.Checks.ExecutorReady, executorReady)

	status := planStatusReady
	if summary.BlockingCount > 0 {
		status = planStatusDraft
	}
	return summary, status, nil
}

func (s *Service) buildSelectionPrecheck(ctx context.Context, session sessionRow, plan planRow, selections []selectionRow) (importdto.PrecheckSummary, string, error) {
	now := s.now().UTC().Format(time.RFC3339)
	summary := importdto.PrecheckSummary{
		UpdatedAt: now,
		Items:     []importdto.PrecheckItem{},
	}
	applyCheck := func(field *string, status string) {
		*field = status
		switch status {
		case "blocking":
			summary.BlockingCount++
		case "risk":
			summary.RiskCount++
		default:
			summary.PassedCount++
		}
	}

	sourceReadable := "passed"
	deviceOnline := "passed"
	executorReady := "passed"
	targetWritable := "passed"
	capacityReady := "passed"
	pathConflict := "passed"

	if session.SessionStatus == sessionStateDisconnected {
		deviceOnline = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{ID: "device-online", Label: "来源设备在线", Status: "blocking", Detail: "来源设备已断开，请重新连接后再继续。"})
	}
	if strings.TrimSpace(session.CallbackBaseURL) == "" {
		executorReady = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{ID: "executor-ready", Label: "本地执行器可用", Status: "blocking", Detail: "本地执行器未就绪，无法执行导入。"})
	}
	if plan.LibraryID == nil {
		targetWritable = "blocking"
		pathConflict = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{ID: "library-required", Label: "已选择资产库", Status: "blocking", Detail: "请先选择导入目标资产库。"})
	}
	if len(selections) == 0 {
		targetWritable = "blocking"
		summary.Items = append(summary.Items, importdto.PrecheckItem{ID: "selection-required", Label: "已选择导入对象", Status: "blocking", Detail: "请先选择要导入的文件夹或文件。"})
	}
	for _, selection := range selections {
		if len(selection.TargetMountIDs) == 0 {
			targetWritable = "blocking"
			summary.Items = append(summary.Items, importdto.PrecheckItem{
				ID:     "target-missing-" + selection.ID,
				Label:  "导入对象目标端配置完整",
				Status: "blocking",
				Detail: fmt.Sprintf("“%s”还没有选择目标端。", selection.Name),
			})
		}
	}

	applyCheck(&summary.Checks.SourceReadable, sourceReadable)
	applyCheck(&summary.Checks.TargetWritable, targetWritable)
	applyCheck(&summary.Checks.CapacityReady, capacityReady)
	applyCheck(&summary.Checks.PathConflict, pathConflict)
	applyCheck(&summary.Checks.DeviceOnline, deviceOnline)
	applyCheck(&summary.Checks.ExecutorReady, executorReady)
	status := planStatusReady
	if summary.BlockingCount > 0 {
		status = planStatusDraft
	}
	return summary, status, nil
}

type importFileCandidate struct {
	RelativePath string
	PhysicalPath string
}

func (s *Service) expandDirectorySelection(ctx context.Context, session sessionRow, relativePath string) ([]importFileCandidate, error) {
	offset := 0
	limit := 500
	items := make([]importFileCandidate, 0)
	for {
		path := relativePath
		response, err := s.bridge.BrowseSource(ctx, session.CallbackBaseURL, importdto.BrowseRequest{
			SourcePath:   session.SourcePath,
			RelativePath: &path,
			Limit:        limit,
			Offset:       offset,
		})
		if err != nil {
			return nil, err
		}
		for _, entry := range response.Entries {
			if entry.EntryType == "DIRECTORY" {
				descendants, err := s.expandDirectorySelection(ctx, session, entry.RelativePath)
				if err != nil {
					return nil, err
				}
				items = append(items, descendants...)
				continue
			}
			items = append(items, importFileCandidate{
				RelativePath: entry.RelativePath,
				PhysicalPath: filepath.Join(session.SourcePath, filepath.FromSlash(entry.RelativePath)),
			})
		}
		if !response.HasMore {
			break
		}
		offset += response.Limit
	}
	return items, nil
}

func (s *Service) detectPathConflict(ctx context.Context, libraryID string, logicalPath string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM assets
			WHERE library_id = $1
			  AND relative_path = $2
			  AND lifecycle_state = 'ACTIVE'
		)
	`, libraryID, normalizeLogicalPath(logicalPath)).Scan(&exists)
	return exists, err
}

type draftTargetSummary struct {
	fileIDs   []string
	targetIDs []string
}

func (s *Service) loadLibraries(ctx context.Context) ([]importdto.LibraryOption, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM libraries ORDER BY created_at ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]importdto.LibraryOption, 0)
	for rows.Next() {
		var item importdto.LibraryOption
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadDrafts(ctx context.Context) ([]importdto.DraftRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, session_id, library_id, status, target_strategy, precheck_summary, updated_at, has_blocking_issues
		FROM import_plans
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entryTargets, err := s.loadDraftTargetSummaries(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]importdto.DraftRecord, 0)
	for rows.Next() {
		var (
			id                string
			sessionID         string
			libraryID         *string
			status            string
			targetStrategy    string
			precheckJSON      []byte
			updatedAt         time.Time
			hasBlockingIssues bool
		)
		if err := rows.Scan(&id, &sessionID, &libraryID, &status, &targetStrategy, &precheckJSON, &updatedAt, &hasBlockingIssues); err != nil {
			return nil, err
		}
		precheck := emptyPrecheckSummary(updatedAt)
		if len(precheckJSON) > 0 && string(precheckJSON) != "{}" {
			_ = json.Unmarshal(precheckJSON, &precheck)
		}
		items = append(items, importdto.DraftRecord{
			ID:                id,
			DeviceSessionID:   sessionID,
			LibraryID:         libraryID,
			SelectedFileIDs:   nonNilStrings(entryTargets[sessionID].fileIDs),
			TargetEndpointIDs: nonNilStrings(entryTargets[sessionID].targetIDs),
			TargetStrategy:    targetStrategy,
			PrecheckSummary:   precheck,
			LastEditedAt:      formatRelativeTimestamp(updatedAt),
			HasBlockingIssues: hasBlockingIssues,
			Status:            mapPlanStatus(status),
		})
	}
	return items, rows.Err()
}

func (s *Service) loadDraftTargetSummaries(ctx context.Context) (map[string]draftTargetSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT session_id, id, target_mount_ids
		FROM import_session_entries
		WHERE entry_type = 'FILE'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]draftTargetSummary)
	for rows.Next() {
		var sessionID string
		var fileID string
		var targetIDs []string
		if err := rows.Scan(&sessionID, &fileID, &targetIDs); err != nil {
			return nil, err
		}
		item := result[sessionID]
		if len(targetIDs) > 0 {
			item.fileIDs = append(item.fileIDs, fileID)
		}
		for _, targetID := range targetIDs {
			if !slices.Contains(item.targetIDs, targetID) {
				item.targetIDs = append(item.targetIDs, targetID)
			}
		}
		result[sessionID] = item
	}
	return result, rows.Err()
}

func (s *Service) loadDevices(
	ctx context.Context,
	drafts []importdto.DraftRecord,
	targetsByLibrary map[string][]string,
	allTargets []importdto.TargetEndpointRecord,
) ([]importdto.DeviceSessionRecord, error) {
	draftBySession := make(map[string]importdto.DraftRecord, len(drafts))
	allTargetIDs := make([]string, 0, len(allTargets))
	for _, target := range allTargets {
		allTargetIDs = append(allTargetIDs, target.ID)
	}
	for _, draft := range drafts {
		draftBySession[draft.DeviceSessionID] = draft
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			s.id, s.agent_id, s.device_key, s.device_label, s.device_type, s.source_path, s.mount_path,
			s.scan_status, s.session_status, s.last_error_code, s.last_error_message,
			s.connected_at, s.last_seen_at, s.disconnected_at, s.capacity_bytes, s.available_bytes,
			a.callback_base_url
		FROM import_sessions s
		INNER JOIN agents a ON a.agent_id = s.agent_id
		ORDER BY s.updated_at DESC, s.connected_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]importdto.DeviceSessionRecord, 0)
	for rows.Next() {
		var row sessionRow
		if err := rows.Scan(
			&row.ID, &row.AgentID, &row.DeviceKey, &row.DeviceLabel, &row.DeviceType, &row.SourcePath, &row.MountPath,
			&row.ScanStatus, &row.SessionStatus, &row.LastErrorCode, &row.LastErrorMessage,
			&row.ConnectedAt, &row.LastSeenAt, &row.DisconnectedAt, &row.CapacityBytes, &row.AvailableBytes,
			&row.CallbackBaseURL,
		); err != nil {
			return nil, err
		}
		draft, hasDraft := draftBySession[row.ID]
		targetIDs := []string{}
		var libraryID *string
		if hasDraft {
			libraryID = draft.LibraryID
			if draft.LibraryID != nil {
				targetIDs = append([]string(nil), targetsByLibrary[*draft.LibraryID]...)
			}
		}
		var activeDraftID *string
		if hasDraft {
			activeDraftID = ptr(draft.ID)
		}
		reportID, _ := s.loadLatestReportIDBySession(ctx, row.ID)
		items = append(items, importdto.DeviceSessionRecord{
			ID:                        row.ID,
			DeviceKey:                 row.DeviceKey,
			DeviceLabel:               row.DeviceLabel,
			DeviceType:                row.DeviceType,
			LibraryID:                 libraryID,
			MountPath:                 row.MountPath,
			ConnectedAt:               formatRelativeTimestamp(row.ConnectedAt),
			ConnectedAtSortKey:        row.ConnectedAt.UnixMilli(),
			LastSeenAt:                formatRelativeTimestamp(row.LastSeenAt),
			CapacitySummary:           buildCapacitySummary(row.CapacityBytes, row.AvailableBytes),
			ScanStatus:                mapScanStatus(row.ScanStatus),
			SessionStatus:             mapSessionStatus(row.SessionStatus),
			ActiveDraftID:             activeDraftID,
			LatestReportID:            reportID,
			IssueIDs:                  []string{},
			DuplicateCount:            0,
			ExceptionCount:            0,
			Description:               row.MountPath,
			AvailableTargetEndpointIDs: targetIDs,
		})
	}
	return items, rows.Err()
}

func (s *Service) loadLatestReportIDBySession(ctx context.Context, sessionID string) (*string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		SELECT id
		FROM import_reports
		WHERE session_id = $1
		ORDER BY latest_updated_at DESC
		LIMIT 1
	`, sessionID).Scan(&id)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (s *Service) loadReports(ctx context.Context) ([]importdto.ReportSnapshot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, session_id, library_id, job_id, title, status, submitted_at, finished_at,
		       success_count, failed_count, partial_count, verify_summary, target_summaries, issue_ids, latest_updated_at, file_count, note
		FROM import_reports
		ORDER BY latest_updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]importdto.ReportSnapshot, 0)
	for rows.Next() {
		var item importdto.ReportSnapshot
		var targetJSON []byte
		var submittedAt time.Time
		var finishedAt *time.Time
		var latestUpdatedAt time.Time
		if err := rows.Scan(&item.ID, &item.DeviceSessionID, &item.LibraryID, &item.TaskID, &item.Title, &item.Status, &submittedAt, &finishedAt,
			&item.SuccessCount, &item.FailedCount, &item.PartialCount, &item.VerifySummary, &targetJSON, &item.IssueIDs, &latestUpdatedAt, &item.FileCount, &item.Note); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(targetJSON, &item.TargetSummaries)
		if item.TargetSummaries == nil {
			item.TargetSummaries = []importdto.ReportTargetSummary{}
		}
		if item.IssueIDs == nil {
			item.IssueIDs = []string{}
		}
		item.Status = mapReportStatus(item.Status)
		item.SubmittedAt = formatRelativeTimestamp(submittedAt)
		item.LatestUpdatedAt = formatRelativeTimestamp(latestUpdatedAt)
		if finishedAt != nil {
			finished := formatRelativeTimestamp(*finishedAt)
			item.FinishedAt = &finished
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadTargetEndpoints(ctx context.Context) ([]importdto.TargetEndpointRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			m.id, m.storage_node_id, m.library_id, m.name, sn.node_type, m.mount_mode, mr.available_bytes,
			COALESCE(mr.auth_status, 'UNKNOWN'), COALESCE(mr.health_status, 'UNKNOWN')
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
		  AND m.enabled = true
		  AND m.mount_mode = 'READ_WRITE'
		  AND sn.node_type IN ('LOCAL')
		ORDER BY m.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]importdto.TargetEndpointRecord, 0)
	for rows.Next() {
		var item importdto.TargetEndpointRecord
		var nodeType string
		var mountMode string
		var availableBytes *int64
		var authStatus string
		var healthStatus string
		if err := rows.Scan(&item.ID, &item.EndpointID, &item.LibraryID, &item.Label, &nodeType, &mountMode, &availableBytes, &authStatus, &healthStatus); err != nil {
			return nil, err
		}
		item.Type = mapTargetType(nodeType)
		item.Writable = mountMode == "READ_WRITE"
		item.AvailableSpace = formatOptionalBytes(availableBytes)
		item.StatusLabel = mapTargetStatus(authStatus, healthStatus)
		item.Tone = mapTargetTone(authStatus, healthStatus)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadTargetsForLibrary(ctx context.Context, libraryID string) ([]targetEndpointRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.storage_node_id, m.library_id, m.name, sn.node_type, m.source_path, m.relative_root_path, mr.available_bytes
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
		  AND m.enabled = true
		  AND m.mount_mode = 'READ_WRITE'
		  AND sn.node_type IN ('LOCAL')
		  AND m.library_id = $1
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]targetEndpointRow, 0)
	for rows.Next() {
		var item targetEndpointRow
		if err := rows.Scan(&item.ID, &item.EndpointID, &item.LibraryID, &item.Label, &item.NodeType, &item.SourcePath, &item.RelativeRootPath, &item.AvailableBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadTargetsByIDs(ctx context.Context, ids []string) ([]targetEndpointRow, error) {
	if len(ids) == 0 {
		return []targetEndpointRow{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.storage_node_id, m.library_id, m.name, sn.node_type, m.source_path, m.relative_root_path, mr.available_bytes
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN mount_runtime mr ON mr.mount_id = m.id
		WHERE m.id = ANY($1)
		  AND sn.node_type IN ('LOCAL')
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]targetEndpointRow, 0)
	for rows.Next() {
		var item targetEndpointRow
		if err := rows.Scan(&item.ID, &item.EndpointID, &item.LibraryID, &item.Label, &item.NodeType, &item.SourcePath, &item.RelativeRootPath, &item.AvailableBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadTargetByID(ctx context.Context, id string) (targetEndpointRow, error) {
	targets, err := s.loadTargetsByIDs(ctx, []string{id})
	if err != nil {
		return targetEndpointRow{}, err
	}
	if len(targets) == 0 {
		return targetEndpointRow{}, apperrors.NotFound("目标端不存在")
	}
	return targets[0], nil
}

func (s *Service) loadActiveAgent(ctx context.Context) (agentRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT agent_id, callback_base_url
		FROM agents
		WHERE callback_base_url <> ''
		ORDER BY last_heartbeat_at DESC
		LIMIT 1
	`)
	var result agentRow
	if err := row.Scan(&result.AgentID, &result.CallbackBaseURL); err != nil {
		if err == pgx.ErrNoRows {
			return agentRow{}, apperrors.NotFound("本地执行器尚未注册")
		}
		return agentRow{}, err
	}
	return result, nil
}

func (s *Service) loadPlan(ctx context.Context, id string) (planRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, session_id, library_id, status, target_strategy, destination_root_path, has_blocking_issues, last_prechecked_at, precheck_summary, submitted_at, finished_at, updated_at
		FROM import_plans
		WHERE id = $1
	`, id)
	var plan planRow
	if err := row.Scan(&plan.ID, &plan.SessionID, &plan.LibraryID, &plan.Status, &plan.TargetStrategy, &plan.DestinationRootPath, &plan.HasBlockingIssues, &plan.LastPrecheckedAt, &plan.PrecheckSummary, &plan.SubmittedAt, &plan.FinishedAt, &plan.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return planRow{}, apperrors.NotFound("导入草稿不存在")
		}
		return planRow{}, err
	}
	return plan, nil
}

func (s *Service) loadPlanBySession(ctx context.Context, sessionID string) (planRow, error) {
	var id string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM import_plans WHERE session_id = $1`, sessionID).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return planRow{}, apperrors.NotFound("导入草稿不存在")
		}
		return planRow{}, err
	}
	return s.loadPlan(ctx, id)
}

func (s *Service) loadSession(ctx context.Context, sessionID string) (sessionRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT s.id, s.agent_id, s.device_key, s.device_label, s.device_type, s.source_path, s.mount_path,
		       s.scan_status, s.session_status, s.last_error_code, s.last_error_message,
		       s.connected_at, s.last_seen_at, s.disconnected_at, s.capacity_bytes, s.available_bytes,
		       a.callback_base_url
		FROM import_sessions s
		INNER JOIN agents a ON a.agent_id = s.agent_id
		WHERE s.id = $1
	`, sessionID)
	var result sessionRow
	if err := row.Scan(&result.ID, &result.AgentID, &result.DeviceKey, &result.DeviceLabel, &result.DeviceType, &result.SourcePath, &result.MountPath,
		&result.ScanStatus, &result.SessionStatus, &result.LastErrorCode, &result.LastErrorMessage,
		&result.ConnectedAt, &result.LastSeenAt, &result.DisconnectedAt, &result.CapacityBytes, &result.AvailableBytes,
		&result.CallbackBaseURL); err != nil {
		if err == pgx.ErrNoRows {
			return sessionRow{}, apperrors.NotFound("导入会话不存在")
		}
		return sessionRow{}, err
	}
	return result, nil
}

func (s *Service) loadEntryRow(ctx context.Context, entryID string) (entryRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, session_id, name, relative_path, file_kind, size_bytes, modified_at, target_mount_ids, import_status
		FROM import_session_entries
		WHERE id = $1
	`, entryID)
	var item entryRow
	if err := row.Scan(&item.ID, &item.SessionID, &item.Name, &item.RelativePath, &item.FileKind, &item.SizeBytes, &item.ModifiedAt, &item.TargetMountIDs, &item.ImportStatus); err != nil {
		if err == pgx.ErrNoRows {
			return entryRow{}, apperrors.NotFound("导入文件不存在")
		}
		return entryRow{}, err
	}
	return item, nil
}

func (s *Service) loadFileEntriesBySession(ctx context.Context, sessionID string) ([]entryRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, session_id, name, relative_path, file_kind, size_bytes, modified_at, target_mount_ids, import_status
		FROM import_session_entries
		WHERE session_id = $1
		  AND entry_type = 'FILE'
		ORDER BY relative_path ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]entryRow, 0)
	for rows.Next() {
		var item entryRow
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Name, &item.RelativePath, &item.FileKind, &item.SizeBytes, &item.ModifiedAt, &item.TargetMountIDs, &item.ImportStatus); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadSelection(ctx context.Context, id string) (selectionRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, plan_id, session_id, entry_type, relative_path, name, target_mount_ids
		FROM import_plan_items
		WHERE id = $1
	`, id)
	var item selectionRow
	if err := row.Scan(&item.ID, &item.PlanID, &item.SessionID, &item.EntryType, &item.RelativePath, &item.Name, &item.TargetMountIDs); err != nil {
		if err == pgx.ErrNoRows {
			return selectionRow{}, apperrors.NotFound("导入选择不存在")
		}
		return selectionRow{}, err
	}
	return item, nil
}

func (s *Service) loadSelectionsByPlan(ctx context.Context, planID string) ([]selectionRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, plan_id, session_id, entry_type, relative_path, name, target_mount_ids
		FROM import_plan_items
		WHERE plan_id = $1
		ORDER BY relative_path ASC
	`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]selectionRow, 0)
	for rows.Next() {
		var item selectionRow
		if err := rows.Scan(&item.ID, &item.PlanID, &item.SessionID, &item.EntryType, &item.RelativePath, &item.Name, &item.TargetMountIDs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadSelectionsBySession(ctx context.Context, sessionID string) (map[string]selectionRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, plan_id, session_id, entry_type, relative_path, name, target_mount_ids
		FROM import_plan_items
		WHERE session_id = $1
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make(map[string]selectionRow)
	for rows.Next() {
		var item selectionRow
		if err := rows.Scan(&item.ID, &item.PlanID, &item.SessionID, &item.EntryType, &item.RelativePath, &item.Name, &item.TargetMountIDs); err != nil {
			return nil, err
		}
		items[item.RelativePath] = item
	}
	return items, rows.Err()
}

func (s *Service) loadReport(ctx context.Context, reportID string) (importdto.ReportSnapshot, error) {
	reports, err := s.loadReports(ctx)
	if err != nil {
		return importdto.ReportSnapshot{}, err
	}
	for _, report := range reports {
		if report.ID == reportID {
			return report, nil
		}
	}
	return importdto.ReportSnapshot{}, apperrors.NotFound("导入报告不存在")
}

func emptyPrecheckSummary(updatedAt time.Time) importdto.PrecheckSummary {
	summary := importdto.PrecheckSummary{
		UpdatedAt: formatRelativeTimestamp(updatedAt),
		Items:     []importdto.PrecheckItem{},
	}
	summary.Checks.SourceReadable = "passed"
	summary.Checks.TargetWritable = "passed"
	summary.Checks.CapacityReady = "passed"
	summary.Checks.PathConflict = "passed"
	summary.Checks.DeviceOnline = "passed"
	summary.Checks.ExecutorReady = "passed"
	return summary
}

func buildCapacitySummary(total *int64, available *int64) importdto.CapacitySummary {
	if total == nil || available == nil || *total <= 0 {
		return importdto.CapacitySummary{Total: "—", Available: "—", UsedPercent: 0}
	}
	usedPercent := int(float64(*total-*available) * 100 / float64(*total))
	return importdto.CapacitySummary{
		Total:       formatBytes(*total),
		Available:   formatBytes(*available),
		UsedPercent: usedPercent,
	}
}

func totalSize(entries []entryRow) int64 {
	var sum int64
	for _, entry := range entries {
		sum += entry.SizeBytes
	}
	return sum
}

func resolveSessionStateFromPrecheck(summary importdto.PrecheckSummary) string {
	if summary.BlockingCount > 0 {
		return sessionStateIssue
	}
	return sessionStateReady
}

func mapScanStatus(status string) string {
	switch status {
	case sessionScanReady:
		return "已完成"
	case sessionScanFailed:
		return "扫描失败"
	default:
		return "待识别"
	}
}

func mapSessionStatus(status string) string {
	switch status {
	case sessionStateImporting:
		return "导入中"
	case sessionStatePartial:
		return "部分完成"
	case sessionStateIssue:
		return "异常待处理"
	case sessionStateDisconnected:
		return "已拔出"
	default:
		return "可导入"
	}
}

func mapPlanStatus(status string) string {
	switch status {
	case planStatusSubmitted, planStatusImporting:
		return "导入中"
	case planStatusCompleted, planStatusFailed:
		return "已提交"
	default:
		return "草稿中"
	}
}

func mapEntryStatus(status string) string {
	switch status {
	case entryStatusQueued:
		return "已排队"
	case entryStatusCompleted:
		return "已完成"
	case entryStatusFailed:
		return "失败"
	case entryStatusConflict:
		return "冲突"
	case entryStatusSkipped:
		return "已跳过"
	default:
		return "待导入"
	}
}

func mapReportStatus(status string) string {
	switch status {
	case reportStatusRunning:
		return "运行中"
	case reportStatusPartial:
		return "部分成功"
	case reportStatusFailed:
		return "失败"
	case reportStatusCompleted:
		return "已完成"
	default:
		return "已排队"
	}
}

func mapSelectionTitleType(entryType string) string {
	if entryType == "DIRECTORY" {
		return "目录"
	}
	return "文件"
}

func mapTargetType(nodeType string) string {
	if nodeType == "NAS" {
		return "NAS/SMB"
	}
	return "本机磁盘"
}

func mapTargetStatus(authStatus string, healthStatus string) string {
	if authStatus == "FAILED" {
		return "鉴权异常"
	}
	if healthStatus == "ERROR" {
		return "连接异常"
	}
	return "可用"
}

func mapTargetTone(authStatus string, healthStatus string) string {
	if authStatus == "FAILED" || healthStatus == "ERROR" {
		return "critical"
	}
	return "success"
}

func formatBytes(value int64) string {
	if value >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(value)/float64(1024*1024*1024))
	}
	if value >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(value)/float64(1024*1024))
	}
	if value >= 1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	return fmt.Sprintf("%d B", value)
}

func formatOptionalBytes(value *int64) string {
	if value == nil {
		return "—"
	}
	return formatBytes(*value)
}

func formatRelativeTimestamp(ts time.Time) string {
	return ts.UTC().Format(time.RFC3339)
}

func parseRFC3339OrNow(value string, fallback time.Time) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed.UTC()
}

func joinLogicalPath(base string, relative string) string {
	base = normalizeLogicalPath(base)
	relative = strings.Trim(strings.ReplaceAll(relative, "\\", "/"), "/")
	if relative == "" {
		return base
	}
	if base == "/" {
		return "/" + relative
	}
	return base + "/" + relative
}

func parentLogicalPath(path string) string {
	normalized := normalizeLogicalPath(path)
	if normalized == "/" {
		return "/"
	}
	index := strings.LastIndex(normalized, "/")
	if index <= 0 {
		return "/"
	}
	return normalized[:index]
}

func parentLogicalPathOrNil(path string) *string {
	parent := parentLogicalPath(path)
	if parent == "/" {
		return nil
	}
	return &parent
}

func normalizeLogicalPath(path string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if normalized == "" || normalized == "/" {
		return "/"
	}
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return "/"
	}
	return "/" + normalized
}

func resolvePhysicalImportPath(sourcePath string, logicalPath string) string {
	normalized := normalizeLogicalPath(logicalPath)
	if normalized == "/" {
		return sourcePath
	}
	return filepath.Join(sourcePath, filepath.FromSlash(strings.TrimPrefix(normalized, "/")))
}

func classifyFileKindFromPath(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".arw", ".cr3", ".nef":
		return "图片"
	case ".mp4", ".mov", ".mxf", ".avi":
		return "视频"
	case ".wav", ".mp3", ".flac", ".aac":
		return "音频"
	case ".txt", ".md", ".pdf", ".doc", ".docx":
		return "文档"
	default:
		return "文件"
	}
}

func valueOrDefault(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func keysOf(items map[string]struct{}) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return []string{""}
	}
	return keys
}

func nonNilStrings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func ptr[T any](value T) *T {
	return &value
}

func optionalPath(path string) *string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return &path
}

func buildCode(prefix string, now time.Time) string {
	var raw [6]byte
	_, _ = rand.Read(raw[:])
	return fmt.Sprintf("%s-%s-%s", prefix, now.UTC().Format("20060102150405"), hex.EncodeToString(raw[:]))
}

func errorsAsNotFound(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "尚未注册")
}
