package issues_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	"mare/services/center/internal/issues"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/notifications"
	"mare/services/center/internal/storage"
	issuedto "mare/shared/contracts/dto/issue"
	storagedto "mare/shared/contracts/dto/storage"
)

const developmentDatabaseURL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"

func TestServiceCreatesIssueForFailedStorageScanAndResolvesAfterRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	localFolders := storage.NewLocalFolderService(pool)
	node, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "本地素材盘",
		RootPath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("save local node: %v", err)
	}

	mount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "商业摄影原片库",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          node.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "raw",
	})
	if err != nil {
		t.Fatalf("save mount: %v", err)
	}

	jobService := jobs.NewService(pool)
	issueService := issues.NewService(pool, jobService)
	jobService.SetIssueSynchronizer(issueService)

	failFirstAttempt := true
	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if failFirstAttempt {
			failFirstAttempt = false
			return fmt.Errorf("远端目录读取超时")
		}
		return nil
	})
	jobService.Start(ctx)

	result, err := jobService.CreateJob(ctx, jobs.CreateJobInput{
		LibraryID:      ptr("photo"),
		JobFamily:      jobs.JobFamilyMaintenance,
		JobIntent:      jobs.JobIntentScanDirectory,
		Title:          "扫描挂载：商业摄影原片库",
		Summary:        "扫描挂载失败后应进入异常中心",
		SourceDomain:   jobs.SourceDomainStorageNodes,
		SourceSnapshot: map[string]any{"libraryName": "商业摄影资产库", "relativePath": "/"},
		Priority:       jobs.PriorityNormal,
		CreatedByType:  jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{
				ItemKey:    "mount:" + mount.Record.ID,
				ItemType:   jobs.ItemTypeDirectoryScan,
				Title:      "扫描挂载：商业摄影原片库",
				Summary:    "扫描挂载目录",
				TargetPath: ptr(mount.Record.Address),
				Links: []jobs.CreateObjectLinkInput{
					{LinkRole: jobs.LinkRoleTargetMount, ObjectType: jobs.ObjectTypeMount, MountID: ptr(mount.Record.ID)},
					{LinkRole: jobs.LinkRoleTargetStorageNode, ObjectType: jobs.ObjectTypeStorageNode, StorageNodeID: ptr(node.Record.ID)},
				},
			},
		},
		Links: []jobs.CreateObjectLinkInput{
			{LinkRole: jobs.LinkRoleTargetMount, ObjectType: jobs.ObjectTypeMount, MountID: ptr(mount.Record.ID)},
			{LinkRole: jobs.LinkRoleTargetStorageNode, ObjectType: jobs.ObjectTypeStorageNode, StorageNodeID: ptr(node.Record.ID)},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	waitForJobStatus(t, ctx, jobService, result.JobID, jobs.StatusFailed)
	issueList := waitForIssueCount(t, ctx, issueService, 1)
	if issueList.Items[0].Source.EndpointID == nil || *issueList.Items[0].Source.EndpointID != mount.Record.ID {
		t.Fatalf("expected endpoint id %s, got %+v", mount.Record.ID, issueList.Items[0].Source)
	}
	if issueList.Items[0].TaskID == nil || *issueList.Items[0].TaskID != result.JobID {
		t.Fatalf("expected task id %s, got %+v", result.JobID, issueList.Items[0])
	}
	if issueList.Items[0].SourceDomain != issues.SourceDomainStorage {
		t.Fatalf("expected source domain %s, got %s", issues.SourceDomainStorage, issueList.Items[0].SourceDomain)
	}

	filtered, err := issueService.ListIssues(ctx, issues.ListQuery{EndpointID: mount.Record.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list issues by endpoint: %v", err)
	}
	if len(filtered.Items) != 1 {
		t.Fatalf("expected one endpoint-filtered issue, got %+v", filtered.Items)
	}

	if _, err := issueService.ApplyAction(ctx, issues.ActionRequest{IDs: []string{issueList.Items[0].ID}, Action: issues.ActionRetry}); err != nil {
		t.Fatalf("retry issue: %v", err)
	}

	waitForJobStatus(t, ctx, jobService, result.JobID, jobs.StatusCompleted)
	waitForNoActiveIssues(t, ctx, issueService)
}

func TestServiceListsIssuesByJobIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	jobService := jobs.NewService(pool)
	issueService := issues.NewService(pool, jobService)
	jobService.SetIssueSynchronizer(issueService)
	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		return fmt.Errorf("扫描失败")
	})
	jobService.Start(ctx)

	result, err := jobService.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "任务异常摘要",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "dir:/", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描目录：/", Summary: "失败"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	waitForJobStatus(t, ctx, jobService, result.JobID, jobs.StatusFailed)

	items, err := issueService.ListByJobIDs(ctx, []string{result.JobID})
	if err != nil {
		t.Fatalf("list issues by job ids: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one job-linked issue, got %+v", items)
	}
}

func TestServiceSyncMissingReplicaIssuesCreatesIssueAndNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at)
		VALUES ('photo', 'library-photo', '商业摄影资产库', '/', 'ACTIVE', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO library_directories (id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at)
		VALUES ('dir-root', 'photo', '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert root directory: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (
			id, code, name, node_type, address, access_mode, enabled, created_at, updated_at
		) VALUES (
			'local-node-1', 'local-node-1', '本地节点', 'LOCAL', 'C:/mare', 'DIRECT', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert storage node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_runtime (
			id, storage_node_id, health_status, auth_status, created_at, updated_at
		) VALUES (
			'local-node-runtime-1', 'local-node-1', 'ONLINE', 'NOT_REQUIRED', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert storage runtime: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-1', 'mount-1', 'photo', '商业摄影资产库', 'local-node-1', '本地挂载', 'LOCAL_PATH', 'READ_ONLY',
			'C:/mare/assets', '/', 'NEVER', 'MANUAL', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-1', 'mount-1', 'SUCCESS', 'NOT_REQUIRED', 'ONLINE', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert mount runtime: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO assets (
			id, library_id, directory_id, relative_path, name, extension, size_bytes, file_kind, lifecycle_state,
			rating, color_label, created_at, updated_at
		) VALUES (
			'asset-1', 'photo', 'dir-root', '/lost-file.jpg', 'lost-file.jpg', 'jpg', 1024, 'IMAGE', 'ACTIVE',
			0, 'NONE', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	missingAt := now.Add(-time.Minute)
	if _, err := pool.Exec(ctx, `
		INSERT INTO asset_replicas (
			id, asset_id, mount_id, physical_path, size_bytes, modified_at, replica_state, sync_state, verification_state,
			last_seen_at, missing_detected_at, created_at, updated_at
		) VALUES (
			'replica-1', 'asset-1', 'mount-1', 'C:/mare/assets/lost-file.jpg', 1024, $1, 'MISSING', 'OUT_OF_SYNC', 'UNVERIFIED',
			$1, $2, $1, $1
		)
	`, now, missingAt); err != nil {
		t.Fatalf("insert missing replica: %v", err)
	}

	jobService := jobs.NewService(pool)
	issueService := issues.NewService(pool, jobService)
	notificationService := notifications.NewService(pool)
	issueService.SetNotificationSynchronizer(notificationService)

	if err := issueService.SyncMissingReplicaIssues(ctx); err != nil {
		t.Fatalf("sync missing replica issues: %v", err)
	}

	result, err := issueService.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 issue, got %+v", result.Items)
	}
	if result.Items[0].SourceDomain != issues.SourceDomainGovernance {
		t.Fatalf("expected governance source domain, got %+v", result.Items[0])
	}
	if result.Items[0].Source.EndpointID == nil || *result.Items[0].Source.EndpointID != "mount-1" {
		t.Fatalf("expected mount link in issue source, got %+v", result.Items[0].Source)
	}

	notices, err := notificationService.ListNotifications(ctx, notifications.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(notices.Items) != 1 {
		t.Fatalf("expected 1 notification, got %+v", notices.Items)
	}
	if notices.Items[0].IssueID == nil || *notices.Items[0].IssueID != result.Items[0].ID {
		t.Fatalf("expected issue-backed notification, got %+v", notices.Items[0])
	}
}

func TestServiceMergesNonConsecutiveRepeatedIssuesByStableDedupeKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at) VALUES ('photo', 'library-photo', '商业摄影资产库', '/', 'ACTIVE', NOW(), NOW())`); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO library_directories (id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at)
		VALUES
			('dir-root', 'photo', '/', '商业摄影资产库', NULL, 0, 'MANUAL', 'ACTIVE', '/', NOW(), NOW()),
			('dir-launch', 'photo', '2026/Shanghai Launch', 'Shanghai Launch', '/', 1, 'MANUAL', 'ACTIVE', '2026/shanghai launch', NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert directories: %v", err)
	}

	jobService := jobs.NewService(pool)
	issueService := issues.NewService(pool, jobService)
	jobService.SetIssueSynchronizer(issueService)

	messages := []string{"conn busy", "permission denied", "conn busy"}
	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if len(messages) == 0 {
			return nil
		}
		message := messages[0]
		messages = messages[1:]
		return fmt.Errorf("%s", message)
	})
	jobService.Start(ctx)

	for i := 0; i < 3; i++ {
		result, err := jobService.CreateJob(ctx, jobs.CreateJobInput{
			LibraryID:      ptr("photo"),
			JobFamily:      jobs.JobFamilyMaintenance,
			JobIntent:      jobs.JobIntentScanDirectory,
			Title:          "扫描目录：/2026/Shanghai Launch",
			Summary:        "测试异常去重",
			SourceDomain:   jobs.SourceDomainFileCenter,
			SourceSnapshot: map[string]any{"libraryName": "商业摄影资产库", "directoryId": "dir-launch", "path": "2026/Shanghai Launch"},
			Priority:       jobs.PriorityNormal,
			CreatedByType:  jobs.CreatedByUser,
			Items: []jobs.CreateItemInput{
				{
					ItemKey:    fmt.Sprintf("dir:dir-launch:%d", i),
					ItemType:   jobs.ItemTypeDirectoryScan,
					Title:      "扫描目录：/2026/Shanghai Launch",
					Summary:    "扫描目录",
					TargetPath: ptr("2026/Shanghai Launch"),
					Links: []jobs.CreateObjectLinkInput{
						{LinkRole: jobs.LinkRoleTargetDirectory, ObjectType: jobs.ObjectTypeDirectory, DirectoryID: ptr("dir-launch")},
					},
				},
			},
			Links: []jobs.CreateObjectLinkInput{
				{LinkRole: jobs.LinkRoleTargetDirectory, ObjectType: jobs.ObjectTypeDirectory, DirectoryID: ptr("dir-launch")},
			},
		})
		if err != nil {
			t.Fatalf("create job %d: %v", i, err)
		}
		waitForJobStatus(t, ctx, jobService, result.JobID, jobs.StatusFailed)
	}

	result, err := issueService.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 deduped issues, got %+v", result.Items)
	}

	var repeated *issuedto.Record
	for i := range result.Items {
		if result.Items[i].Summary == "conn busy" {
			repeated = &result.Items[i]
			break
		}
	}
	if repeated == nil {
		t.Fatalf("expected merged conn busy issue, got %+v", result.Items)
	}
	if repeated.OccurrenceCount != 2 {
		t.Fatalf("expected occurrence count 2, got %+v", repeated)
	}
}

func waitForIssueCount(t *testing.T, ctx context.Context, service *issues.Service, expected int) issuedto.ListResponse {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
		if err == nil && len(result.Items) == expected {
			return result
		}
		time.Sleep(100 * time.Millisecond)
	}

	result, err := service.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	t.Fatalf("expected %d issues, got %+v", expected, result.Items)
	return result
}

func waitForNoActiveIssues(t *testing.T, ctx context.Context, service *issues.Service) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ListIssues(ctx, issues.ListQuery{Status: issues.StatusOpen, Page: 1, PageSize: 20})
		if err == nil && len(result.Items) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	result, err := service.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	t.Fatalf("expected no active issues, got %+v", result.Items)
}

func waitForJobStatus(t *testing.T, ctx context.Context, service *jobs.Service, jobID string, expected string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := service.LoadJobDetail(ctx, jobID)
		if err == nil && detail.Job.Status == expected {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	detail, err := service.LoadJobDetail(ctx, jobID)
	if err != nil {
		t.Fatalf("load job detail: %v", err)
	}
	t.Fatalf("expected job status %s, got %+v", expected, detail.Job)
}

func openTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pool, err := db.Open(ctx, isolatedSchemaDatabaseURL(t, ctx))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func isolatedSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	schema := fmt.Sprintf("test_issues_%d", time.Now().UnixNano())
	adminPool, err := pgxpool.New(ctx, developmentDatabaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
	})

	parsed, err := url.Parse(developmentDatabaseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS issue_object_links;
		DROP TABLE IF EXISTS issue_events;
		DROP TABLE IF EXISTS issues;
		DROP TABLE IF EXISTS directory_tag_links;
		DROP TABLE IF EXISTS asset_tag_links;
		DROP TABLE IF EXISTS job_object_links;
		DROP TABLE IF EXISTS job_events;
		DROP TABLE IF EXISTS job_attempts;
		DROP TABLE IF EXISTS job_items;
		DROP TABLE IF EXISTS jobs;
		DROP TABLE IF EXISTS tag_library_scopes;
		DROP TABLE IF EXISTS tags;
		DROP TABLE IF EXISTS tag_groups;
		DROP TABLE IF EXISTS asset_metadata;
		DROP TABLE IF EXISTS directory_presences;
		DROP TABLE IF EXISTS asset_replicas;
		DROP TABLE IF EXISTS assets;
		DROP TABLE IF EXISTS library_directories;
		DROP TABLE IF EXISTS libraries;
		DROP TABLE IF EXISTS mount_scan_histories;
		DROP TABLE IF EXISTS mount_runtime;
		DROP TABLE IF EXISTS mounts;
		DROP TABLE IF EXISTS storage_node_runtime;
		DROP TABLE IF EXISTS storage_node_credentials;
		DROP TABLE IF EXISTS storage_nodes;
		DROP TABLE IF EXISTS agents;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

func ptr(value string) *string {
	return &value
}
