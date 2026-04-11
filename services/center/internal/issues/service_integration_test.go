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
