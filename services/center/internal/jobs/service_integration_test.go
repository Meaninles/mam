package jobs_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/assets"
	"mare/services/center/internal/db"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/storage"
	assetdto "mare/shared/contracts/dto/asset"
	jobdto "mare/shared/contracts/dto/job"
	storagedto "mare/shared/contracts/dto/storage"
)

const developmentDatabaseURL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"

func TestServiceExecutesCreatedJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := jobs.NewService(pool)
	service.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		return nil
	})
	service.Start(ctx)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "测试作业",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "mount:a", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 A", Summary: "扫描 A"},
			{ItemKey: "mount:b", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 B", Summary: "扫描 B"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	detail := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	if detail.Job.SuccessItems != 2 {
		t.Fatalf("expected two successful items, got %+v", detail.Job)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("expected two items, got %+v", detail.Items)
	}

	events, err := service.ListJobEvents(ctx, result.JobID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events.Items) < 4 {
		t.Fatalf("expected event stream, got %+v", events.Items)
	}
}

func TestServicePauseAndResumeJob(t *testing.T) {
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

	service := jobs.NewService(pool)
	firstItemRelease := make(chan struct{})
	service.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if execution.Item.ItemKey == "mount:first" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-firstItemRelease:
			}
		}
		return nil
	})
	service.Start(ctx)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "暂停恢复测试",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "mount:first", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 first", Summary: "扫描 first"},
			{ItemKey: "mount:second", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 second", Summary: "扫描 second"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	waitForItemStatus(t, ctx, service, result.JobID, "mount:first", jobs.ItemStatusRunning)
	if _, err := service.PauseJob(ctx, result.JobID); err != nil {
		t.Fatalf("pause job: %v", err)
	}
	close(firstItemRelease)

	paused := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusPaused)
	secondPending := false
	for _, item := range paused.Items {
		if item.ItemKey == "mount:second" && item.Status == jobs.ItemStatusPending {
			secondPending = true
		}
	}
	if !secondPending {
		t.Fatalf("expected second item pending after pause, got %+v", paused.Items)
	}

	if _, err := service.ResumeJob(ctx, result.JobID); err != nil {
		t.Fatalf("resume job: %v", err)
	}
	final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	if final.Job.SuccessItems != 2 {
		t.Fatalf("expected resumed job to finish, got %+v", final.Job)
	}
}

func TestServiceRetryFailedJob(t *testing.T) {
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

	service := jobs.NewService(pool)
	var mu sync.Mutex
	attempts := 0
	service.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts == 1 {
			return fmt.Errorf("first attempt failed")
		}
		return nil
	})
	service.Start(ctx)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "失败重试测试",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "mount:first", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 first", Summary: "扫描 first"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusFailed)
	if _, err := service.RetryJob(ctx, result.JobID); err != nil {
		t.Fatalf("retry job: %v", err)
	}
	final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	if final.Job.SuccessItems != 1 {
		t.Fatalf("expected retry to succeed, got %+v", final.Job)
	}
}

func TestServicePauseAndResumeRunningItem(t *testing.T) {
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

	service := jobs.NewService(pool)
	firstItemRelease := make(chan struct{})
	service.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if execution.Item.ItemKey != "mount:first" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-firstItemRelease:
			return nil
		}
	})
	service.Start(ctx)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "子任务暂停恢复测试",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "mount:first", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 first", Summary: "扫描 first"},
			{ItemKey: "mount:second", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 second", Summary: "扫描 second"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	runningDetail := waitForItemStatus(t, ctx, service, result.JobID, "mount:first", jobs.ItemStatusRunning)
	firstItemID := findItemID(t, runningDetail.Items, "mount:first")

	if _, err := service.PauseJobItem(ctx, firstItemID); err != nil {
		t.Fatalf("pause item: %v", err)
	}

	pausedDetail := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusPaused)
	assertItemStatus(t, pausedDetail.Items, "mount:first", jobs.ItemStatusPaused)
	assertItemStatus(t, pausedDetail.Items, "mount:second", jobs.ItemStatusPending)

	if _, err := service.ResumeJobItem(ctx, firstItemID); err != nil {
		t.Fatalf("resume item: %v", err)
	}

	close(firstItemRelease)
	final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	if final.Job.SuccessItems != 2 {
		t.Fatalf("expected resumed item flow to complete, got %+v", final.Job)
	}
}

func TestServiceCancelRunningItemKeepsRemainingItemsExecutable(t *testing.T) {
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

	service := jobs.NewService(pool)
	service.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if execution.Item.ItemKey != "mount:first" {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	})
	service.Start(ctx)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "子任务取消测试",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "mount:first", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 first", Summary: "扫描 first"},
			{ItemKey: "mount:second", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描 second", Summary: "扫描 second"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	runningDetail := waitForItemStatus(t, ctx, service, result.JobID, "mount:first", jobs.ItemStatusRunning)
	firstItemID := findItemID(t, runningDetail.Items, "mount:first")

	if _, err := service.CancelJobItem(ctx, firstItemID); err != nil {
		t.Fatalf("cancel item: %v", err)
	}

	final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	assertItemStatus(t, final.Items, "mount:first", jobs.ItemStatusCanceled)
	assertItemStatus(t, final.Items, "mount:second", jobs.ItemStatusCompleted)
	if final.Job.SuccessItems != 1 {
		t.Fatalf("expected second item to succeed after cancel, got %+v", final.Job)
	}
}

func TestServiceExecutesReplicateJobAgainstRealAssets(t *testing.T) {
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

	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	localFolders := storage.NewLocalFolderService(pool)

	sourceNode, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "源节点",
		RootPath: sourceRoot,
		Notes:    "",
	})
	if err != nil {
		t.Fatalf("save source node: %v", err)
	}
	targetNode, err := localFolders.SaveLocalNode(ctx, storagedto.SaveLocalNodeRequest{
		Name:     "目标节点",
		RootPath: targetRoot,
		Notes:    "",
	})
	if err != nil {
		t.Fatalf("save target node: %v", err)
	}

	sourceMount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "源挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          sourceNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "source",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save source mount: %v", err)
	}
	targetMount, err := localFolders.SaveLocalFolder(ctx, storagedto.SaveLocalFolderRequest{
		Name:            "目标挂载",
		LibraryID:       "photo",
		LibraryName:     "商业摄影资产库",
		NodeID:          targetNode.Record.ID,
		MountMode:       "可写",
		HeartbeatPolicy: "从不",
		RelativePath:    "target",
		Notes:           "",
	})
	if err != nil {
		t.Fatalf("save target mount: %v", err)
	}

	sourceDir := filepath.Join(sourceRoot, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	sourceFile := filepath.Join(sourceDir, "cover.jpg")
	if err := os.WriteFile(sourceFile, []byte("cover-image"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	expectedModifiedAt := time.Date(2024, 7, 1, 8, 30, 0, 0, time.UTC)
	if err := os.Chtimes(sourceFile, expectedModifiedAt, expectedModifiedAt); err != nil {
		t.Fatalf("set source mtime: %v", err)
	}

	if _, err := localFolders.RunLocalFolderScan(ctx, []string{sourceMount.Record.ID}); err != nil {
		t.Fatalf("run source scan: %v", err)
	}

	assetService := assets.NewService(pool)
	root, err := assetService.BrowseLibrary(ctx, "photo", assetdto.BrowseQuery{
		Page:          1,
		PageSize:      20,
		FileType:      "全部",
		StatusFilter:  "全部",
		SortValue:     "名称",
		SortDirection: "asc",
	})
	if err != nil {
		t.Fatalf("browse library: %v", err)
	}
	if len(root.Items) != 1 {
		t.Fatalf("expected one asset, got %#v", root.Items)
	}
	assetID := root.Items[0].ID

	plan, err := assetService.PrepareReplicatePlan(ctx, assetdto.CreateReplicateJobRequest{
		EntryIDs:     []string{assetID},
		EndpointName: targetMount.Record.Name,
	})
	if err != nil {
		t.Fatalf("prepare replicate plan: %v", err)
	}

	service := jobs.NewService(pool)
	service.RegisterExecutor(jobs.JobIntentReplicate, func(ctx context.Context, execution jobs.ExecutionContext) error {
		var sourceReplicaID string
		var targetMountID string
		for _, link := range execution.ItemLinks {
			if link.AssetReplicaID != nil {
				sourceReplicaID = *link.AssetReplicaID
			}
			if link.MountID != nil && link.LinkRole == jobs.LinkRoleTargetMount {
				targetMountID = *link.MountID
			}
		}
		return assetService.ExecuteReplicaSync(ctx, sourceReplicaID, targetMountID)
	})
	service.Start(ctx)

	result, err := service.CreateReplicateJob(ctx, plan)
	if err != nil {
		t.Fatalf("create replicate job: %v", err)
	}

	detail := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	if detail.Job.JobIntent != jobs.JobIntentReplicate {
		t.Fatalf("expected replicate intent, got %#v", detail.Job)
	}
	if detail.Job.SuccessItems != 1 {
		t.Fatalf("expected one successful item, got %#v", detail.Job)
	}

	targetFile := filepath.Join(targetRoot, "target", "cover.jpg")
	if content, err := os.ReadFile(targetFile); err != nil {
		t.Fatalf("read target file: %v", err)
	} else if string(content) != "cover-image" {
		t.Fatalf("unexpected target file content: %q", string(content))
	}
	if info, err := os.Stat(targetFile); err != nil {
		t.Fatalf("stat target file: %v", err)
	} else if !info.ModTime().UTC().Equal(expectedModifiedAt) {
		t.Fatalf("expected target mtime %s, got %s", expectedModifiedAt.Format(time.RFC3339), info.ModTime().UTC().Format(time.RFC3339))
	}

	entry, err := assetService.LoadEntry(ctx, assetID)
	if err != nil {
		t.Fatalf("load entry after job: %v", err)
	}
	if entry.LastTaskText == "暂无任务" || !strings.Contains(entry.LastTaskText, "同步到") {
		t.Fatalf("expected task linkage in file center entry, got %#v", entry.LastTaskText)
	}
}

func waitForJobStatus(t *testing.T, ctx context.Context, service *jobs.Service, jobID string, expected string) jobdto.Detail {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := service.LoadJobDetail(ctx, jobID)
		if err == nil && detail.Job.Status == expected {
			return detail
		}
		time.Sleep(100 * time.Millisecond)
	}

	detail, err := service.LoadJobDetail(ctx, jobID)
	if err != nil {
		t.Fatalf("load job detail: %v", err)
	}
	t.Fatalf("expected job status %s, got %+v", expected, detail.Job)
	return jobdto.Detail{}
}

func waitForItemStatus(t *testing.T, ctx context.Context, service *jobs.Service, jobID string, itemKey string, expected string) jobdto.Detail {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := service.LoadJobDetail(ctx, jobID)
		if err == nil {
			for _, item := range detail.Items {
				if item.ItemKey == itemKey && item.Status == expected {
					return detail
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("expected item %s status %s", itemKey, expected)
	return jobdto.Detail{}
}

func findItemID(t *testing.T, items []jobdto.ItemRecord, itemKey string) string {
	t.Helper()

	for _, item := range items {
		if item.ItemKey == itemKey {
			return item.ID
		}
	}
	t.Fatalf("expected item %s", itemKey)
	return ""
}

func assertItemStatus(t *testing.T, items []jobdto.ItemRecord, itemKey string, expected string) {
	t.Helper()

	for _, item := range items {
		if item.ItemKey == itemKey {
			if item.Status != expected {
				t.Fatalf("expected item %s status %s, got %s", itemKey, expected, item.Status)
			}
			return
		}
	}
	t.Fatalf("expected item %s", itemKey)
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

	schema := fmt.Sprintf("test_jobs_%d", time.Now().UnixNano())
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
