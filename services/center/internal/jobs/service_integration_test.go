package jobs_test

import (
	"context"
	"encoding/json"
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

	paused := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusPaused)
	assertItemStatus(t, paused.Items, "mount:first", jobs.ItemStatusPaused)
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
	close(firstItemRelease)
	final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
	if final.Job.SuccessItems != 2 {
		t.Fatalf("expected resumed job to finish, got %+v", final.Job)
	}
}

func TestServicePauseAndResumeJobSyncsExternalTaskStatus(t *testing.T) {
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
	controller := &fakeExternalTaskController{
		pauseStatuses:  map[string]string{"gid-aria2-1": "paused"},
		resumeStatuses: map[string]string{"gid-aria2-1": "active"},
	}
	service.SetExternalTaskController(controller)

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
		Summary:       "外部任务状态同步测试",
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
	if err := service.UpdateExternalTask(ctx, result.JobID, firstItemID, "ARIA2", "gid-aria2-1", "active", nil, nil); err != nil {
		t.Fatalf("seed external task state: %v", err)
	}

	if _, err := service.PauseJob(ctx, result.JobID); err != nil {
		t.Fatalf("pause job: %v", err)
	}

	paused := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusPaused)
	assertItemStatus(t, paused.Items, "mount:first", jobs.ItemStatusPaused)
	pausedItem := findItemByKey(t, paused.Items, "mount:first")
	if pausedItem.ExternalTaskStatus == nil || *pausedItem.ExternalTaskStatus != "paused" {
		t.Fatalf("expected paused external status, got %+v", pausedItem.ExternalTaskStatus)
	}
	if len(controller.pausedCalls) != 1 || controller.pausedCalls[0] != "ARIA2:gid-aria2-1" {
		t.Fatalf("expected external pause call, got %#v", controller.pausedCalls)
	}

	if _, err := service.ResumeJob(ctx, result.JobID); err != nil {
		t.Fatalf("resume job: %v", err)
	}
	resumed := waitForItemStatus(t, ctx, service, result.JobID, "mount:first", jobs.ItemStatusRunning)
	resumedItem := findItemByKey(t, resumed.Items, "mount:first")
	if resumedItem.ExternalTaskStatus == nil || *resumedItem.ExternalTaskStatus != "active" {
		t.Fatalf("expected resumed external status, got %+v", resumedItem.ExternalTaskStatus)
	}
	if len(controller.resumedCalls) != 1 || controller.resumedCalls[0] != "ARIA2:gid-aria2-1" {
		t.Fatalf("expected external resume call, got %#v", controller.resumedCalls)
	}
	close(firstItemRelease)
}

func TestServiceCancelJobSyncsExternalTaskStatus(t *testing.T) {
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
	controller := &fakeExternalTaskController{
		cancelStatuses: map[string]string{"gid-aria2-1": "removed"},
	}
	service.SetExternalTaskController(controller)

	release := make(chan struct{})
	service.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	})
	service.Start(ctx)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "外部任务取消同步测试",
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

	runningDetail := waitForItemStatus(t, ctx, service, result.JobID, "mount:first", jobs.ItemStatusRunning)
	itemID := findItemID(t, runningDetail.Items, "mount:first")
	if err := service.UpdateExternalTask(ctx, result.JobID, itemID, "ARIA2", "gid-aria2-1", "active", nil, nil); err != nil {
		t.Fatalf("seed external task state: %v", err)
	}

	if _, err := service.CancelJob(ctx, result.JobID); err != nil {
		t.Fatalf("cancel job: %v", err)
	}

	canceled := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCanceled)
	item := findItemByKey(t, canceled.Items, "mount:first")
	if item.ExternalTaskStatus == nil || *item.ExternalTaskStatus != "removed" {
		t.Fatalf("expected removed external task status, got %+v", item.ExternalTaskStatus)
	}
	if len(controller.canceledCalls) != 1 || controller.canceledCalls[0] != "ARIA2:gid-aria2-1" {
		t.Fatalf("expected external cancel call, got %#v", controller.canceledCalls)
	}
	close(release)
}

func TestServiceCancelJobAlsoStopsQueuedExternalTask(t *testing.T) {
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
	controller := &fakeExternalTaskController{
		cancelStatuses: map[string]string{"gid-aria2-queued": "removed"},
	}
	service.SetExternalTaskController(controller)

	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "排队外部任务取消测试",
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

	detail, err := service.LoadJobDetail(ctx, result.JobID)
	if err != nil {
		t.Fatalf("load job detail: %v", err)
	}
	itemID := findItemID(t, detail.Items, "mount:first")
	if _, err := pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'QUEUED'
		WHERE id = $1
	`, result.JobID); err != nil {
		t.Fatalf("seed queued job: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE job_items
		SET status = 'QUEUED'
		WHERE id = $1
	`, itemID); err != nil {
		t.Fatalf("seed queued item: %v", err)
	}
	if err := service.UpdateExternalTask(ctx, result.JobID, itemID, "ARIA2", "gid-aria2-queued", "active", nil, nil); err != nil {
		t.Fatalf("seed external task state: %v", err)
	}

	if _, err := service.CancelJob(ctx, result.JobID); err != nil {
		t.Fatalf("cancel job: %v", err)
	}

	canceled := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCanceled)
	item := findItemByKey(t, canceled.Items, "mount:first")
	if item.ExternalTaskStatus == nil || *item.ExternalTaskStatus != "removed" {
		t.Fatalf("expected removed external task status, got %+v", item.ExternalTaskStatus)
	}
	if len(controller.canceledCalls) != 1 || controller.canceledCalls[0] != "ARIA2:gid-aria2-queued" {
		t.Fatalf("expected queued external cancel call, got %#v", controller.canceledCalls)
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

func TestServiceRestartRequeuesRunningItems(t *testing.T) {
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
	result, err := service.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/",
		Summary:       "服务重启恢复测试",
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

	detail, err := service.LoadJobDetail(ctx, result.JobID)
	if err != nil {
		t.Fatalf("load job detail: %v", err)
	}
	firstItemID := findItemID(t, detail.Items, "mount:first")
	secondItemID := findItemID(t, detail.Items, "mount:second")

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'RUNNING',
		    started_at = $2,
		    updated_at = $2
		WHERE id = $1
	`, result.JobID, now); err != nil {
		t.Fatalf("seed running job: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE job_items
		SET status = CASE WHEN id = $1 THEN 'RUNNING' ELSE 'PENDING' END,
		    phase = CASE WHEN id = $1 THEN 'EXECUTING' ELSE NULL END,
		    started_at = CASE WHEN id = $1 THEN $3 ELSE started_at END,
		    updated_at = $3
		WHERE id IN ($1, $2)
	`, firstItemID, secondItemID, now); err != nil {
		t.Fatalf("seed running items: %v", err)
	}

	restarted := jobs.NewService(pool)
	executed := make(chan string, 2)
	restarted.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		select {
		case executed <- execution.Item.ItemKey:
		default:
		}
		return nil
	})
	restarted.Start(ctx)

	final := waitForJobStatus(t, ctx, restarted, result.JobID, jobs.StatusCompleted)
	assertItemStatus(t, final.Items, "mount:first", jobs.ItemStatusCompleted)
	assertItemStatus(t, final.Items, "mount:second", jobs.ItemStatusCompleted)

	executedKeys := make(map[string]struct{}, 2)
	deadline := time.After(2 * time.Second)
	for len(executedKeys) < 2 {
		select {
		case key := <-executed:
			executedKeys[key] = struct{}{}
		case <-deadline:
			t.Fatalf("expected both queued and recovered items to execute, got %#v", executedKeys)
		}
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

func TestServicePauseAndResumeReplicateJobsAcrossCloudRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	cases := []struct {
		name              string
		sourceNodeType    string
		targetNodeType    string
		expectedRouteType string
	}{
		{name: "cloud_to_local", sourceNodeType: "CLOUD", targetNodeType: "LOCAL", expectedRouteType: "DOWNLOAD"},
		{name: "cloud_to_nas", sourceNodeType: "CLOUD", targetNodeType: "NAS", expectedRouteType: "DOWNLOAD"},
		{name: "local_to_cloud", sourceNodeType: "LOCAL", targetNodeType: "CLOUD", expectedRouteType: "UPLOAD"},
		{name: "nas_to_cloud", sourceNodeType: "NAS", targetNodeType: "CLOUD", expectedRouteType: "UPLOAD"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			pool := openTestPool(t, ctx)
			defer pool.Close()
			resetSchema(t, ctx, pool)

			migrator := db.NewMigrator()
			if _, err := migrator.Apply(ctx, pool); err != nil {
				t.Fatalf("apply migrations: %v", err)
			}

			plan := createReplicatePlanForRoute(t, ctx, pool, tc.sourceNodeType, tc.targetNodeType)
			if len(plan.Items) != 1 {
				t.Fatalf("expected one plan item, got %#v", plan)
			}
			if plan.Items[0].RouteType != tc.expectedRouteType {
				t.Fatalf("expected item route type %s, got %#v", tc.expectedRouteType, plan.Items[0])
			}

			service := jobs.NewService(pool)
			release := make(chan struct{})
			service.RegisterExecutor(jobs.JobIntentReplicate, func(ctx context.Context, execution jobs.ExecutionContext) error {
				if execution.Item.RouteType == nil || *execution.Item.RouteType != tc.expectedRouteType {
					return fmt.Errorf("unexpected route type: %v", execution.Item.RouteType)
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
					return nil
				}
			})
			service.Start(ctx)

			result, err := service.CreateReplicateJob(ctx, plan)
			if err != nil {
				t.Fatalf("create replicate job: %v", err)
			}

			runningDetail := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusRunning)
			itemKey := runningDetail.Items[0].ItemKey
			if _, err := service.PauseJob(ctx, result.JobID); err != nil {
				t.Fatalf("pause route %s job: %v", tc.name, err)
			}

			pausedDetail := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusPaused)
			if pausedDetail.Job.RouteType == nil || *pausedDetail.Job.RouteType != tc.expectedRouteType {
				t.Fatalf("expected paused job route type %s, got %#v", tc.expectedRouteType, pausedDetail.Job)
			}
			assertItemStatus(t, pausedDetail.Items, itemKey, jobs.ItemStatusPaused)

			if _, err := service.ResumeJob(ctx, result.JobID); err != nil {
				t.Fatalf("resume route %s job: %v", tc.name, err)
			}

			close(release)
			waitForItemStatus(t, ctx, service, result.JobID, itemKey, jobs.ItemStatusCompleted)
			final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCompleted)
			if final.Job.RouteType == nil || *final.Job.RouteType != tc.expectedRouteType {
				t.Fatalf("expected completed job route type %s, got %#v", tc.expectedRouteType, final.Job)
			}
			assertItemStatus(t, final.Items, itemKey, jobs.ItemStatusCompleted)
		})
	}
}

func TestServiceCancelReplicateJobsAcrossCloudRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	cases := []struct {
		name              string
		sourceNodeType    string
		targetNodeType    string
		expectedRouteType string
	}{
		{name: "cloud_to_local", sourceNodeType: "CLOUD", targetNodeType: "LOCAL", expectedRouteType: "DOWNLOAD"},
		{name: "cloud_to_nas", sourceNodeType: "CLOUD", targetNodeType: "NAS", expectedRouteType: "DOWNLOAD"},
		{name: "local_to_cloud", sourceNodeType: "LOCAL", targetNodeType: "CLOUD", expectedRouteType: "UPLOAD"},
		{name: "nas_to_cloud", sourceNodeType: "NAS", targetNodeType: "CLOUD", expectedRouteType: "UPLOAD"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			pool := openTestPool(t, ctx)
			defer pool.Close()
			resetSchema(t, ctx, pool)

			migrator := db.NewMigrator()
			if _, err := migrator.Apply(ctx, pool); err != nil {
				t.Fatalf("apply migrations: %v", err)
			}

			plan := createReplicatePlanForRoute(t, ctx, pool, tc.sourceNodeType, tc.targetNodeType)
			if len(plan.Items) != 1 {
				t.Fatalf("expected one plan item, got %#v", plan)
			}

			service := jobs.NewService(pool)
			service.RegisterExecutor(jobs.JobIntentReplicate, func(ctx context.Context, execution jobs.ExecutionContext) error {
				if execution.Item.RouteType == nil || *execution.Item.RouteType != tc.expectedRouteType {
					return fmt.Errorf("unexpected route type: %v", execution.Item.RouteType)
				}
				<-ctx.Done()
				return ctx.Err()
			})
			service.Start(ctx)

			result, err := service.CreateReplicateJob(ctx, plan)
			if err != nil {
				t.Fatalf("create replicate job: %v", err)
			}

			runningDetail := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusRunning)
			itemKey := runningDetail.Items[0].ItemKey

			if _, err := service.CancelJob(ctx, result.JobID); err != nil {
				t.Fatalf("cancel route %s job: %v", tc.name, err)
			}

			waitForItemStatus(t, ctx, service, result.JobID, itemKey, jobs.ItemStatusCanceled)
			final := waitForJobStatus(t, ctx, service, result.JobID, jobs.StatusCanceled)
			if final.Job.RouteType == nil || *final.Job.RouteType != tc.expectedRouteType {
				t.Fatalf("expected canceled job route type %s, got %#v", tc.expectedRouteType, final.Job)
			}
			assertItemStatus(t, final.Items, itemKey, jobs.ItemStatusCanceled)
		})
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

type fakeExternalTaskController struct {
	pauseStatuses  map[string]string
	resumeStatuses map[string]string
	cancelStatuses map[string]string
	pausedCalls    []string
	resumedCalls   []string
	canceledCalls  []string
}

func (f *fakeExternalTaskController) PauseExternalTask(ctx context.Context, engine string, taskID string) (string, error) {
	f.pausedCalls = append(f.pausedCalls, engine+":"+taskID)
	if status, ok := f.pauseStatuses[taskID]; ok {
		return status, nil
	}
	return "paused", nil
}

func (f *fakeExternalTaskController) ResumeExternalTask(ctx context.Context, engine string, taskID string) (string, error) {
	f.resumedCalls = append(f.resumedCalls, engine+":"+taskID)
	if status, ok := f.resumeStatuses[taskID]; ok {
		return status, nil
	}
	return "active", nil
}

func (f *fakeExternalTaskController) CancelExternalTask(ctx context.Context, engine string, taskID string) (string, error) {
	f.canceledCalls = append(f.canceledCalls, engine+":"+taskID)
	if status, ok := f.cancelStatuses[taskID]; ok {
		return status, nil
	}
	return "removed", nil
}

func createReplicatePlanForRoute(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	sourceNodeType string,
	targetNodeType string,
) assets.ReplicatePlan {
	t.Helper()

	now := time.Now().UTC()
	libraryID := fmt.Sprintf("photo_%s_%s", strings.ToLower(sourceNodeType), strings.ToLower(targetNodeType))
	rootDirID := "dir-root-" + libraryID
	assetID := "asset-" + libraryID
	sourceNodeID := "node-source-" + libraryID
	targetNodeID := "node-target-" + libraryID
	sourceMountID := "mount-source-" + libraryID
	targetMountID := "mount-target-" + libraryID
	sourceReplicaID := "replica-source-" + libraryID
	relativePath := "cloud-route/clip.mov"
	sourcePath := filepath.Join(t.TempDir(), "source")
	targetPath := filepath.Join(t.TempDir(), "target")
	if sourceNodeType == "CLOUD" {
		sourcePath = "/MareArchive/" + libraryID + "/source"
	}
	if targetNodeType == "CLOUD" {
		targetPath = "/MareArchive/" + libraryID + "/target"
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at)
		VALUES ($1, $2, $3, '/', 'ACTIVE', $4, $4)
	`, libraryID, "library-"+libraryID, "商业摄影资产库", now); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO library_directories (
			id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
		) VALUES ($1, $2, '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $3, $3)
	`, rootDirID, libraryID, now); err != nil {
		t.Fatalf("insert root directory: %v", err)
	}

	insertStorageNodeForRoute(t, ctx, pool, sourceNodeID, sourceNodeType, sourcePath, now)
	insertStorageNodeForRoute(t, ctx, pool, targetNodeID, targetNodeType, targetPath, now)
	insertMountForRoute(t, ctx, pool, libraryID, sourceMountID, sourceNodeID, "源端点", sourceNodeType, sourcePath, now)
	insertMountForRoute(t, ctx, pool, libraryID, targetMountID, targetNodeID, "目标端点", targetNodeType, targetPath, now)

	if _, err := pool.Exec(ctx, `
		INSERT INTO assets (
			id, library_id, directory_id, relative_path, name, extension, size_bytes, mime_type,
			file_kind, lifecycle_state, rating, color_label, note, canonical_modified_at,
			content_changed_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, 'clip.mov', '.mov', 1048576, 'video/quicktime',
			'VIDEO', 'ACTIVE', 0, 'NONE', NULL, $5,
			$5, $5, $5
		)
	`, assetID, libraryID, rootDirID, relativePath, now); err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO asset_replicas (
			id, asset_id, mount_id, physical_path, size_bytes, modified_at, replica_state, sync_state,
			verification_state, last_seen_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, 1048576, $5, 'AVAILABLE', 'IN_SYNC',
			'UNVERIFIED', $5, $5, $5
		)
	`, sourceReplicaID, assetID, sourceMountID, buildReplicaPhysicalPath(sourceNodeType, sourcePath, relativePath), now); err != nil {
		t.Fatalf("insert source replica: %v", err)
	}

	assetService := assets.NewService(pool)
	plan, err := assetService.PrepareReplicatePlan(ctx, assetdto.CreateReplicateJobRequest{
		EntryIDs:     []string{assetID},
		EndpointName: "目标端点",
	})
	if err != nil {
		t.Fatalf("prepare replicate plan: %v", err)
	}
	return plan
}

func insertStorageNodeForRoute(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	nodeID string,
	nodeType string,
	basePath string,
	now time.Time,
) {
	t.Helper()

	switch nodeType {
	case "LOCAL":
		if _, err := pool.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, access_mode, enabled, created_at, updated_at
			) VALUES (
				$1, $1, $2, 'LOCAL', 'DIRECT', true, $3, $3
			)
		`, nodeID, "本地节点", now); err != nil {
			t.Fatalf("insert local node: %v", err)
		}
	case "NAS":
		if _, err := pool.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, address, access_mode, account_alias, enabled, description, created_at, updated_at
			) VALUES (
				$1, $1, $2, 'NAS', $3, 'SMB', 'mare', true, 'nas route test', $4, $4
			)
		`, nodeID, "NAS 节点", basePath, now); err != nil {
			t.Fatalf("insert nas node: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO storage_node_credentials (
				id, storage_node_id, credential_kind, username, secret_ciphertext, updated_at, created_at
			) VALUES (
				$1, $2, 'PASSWORD', 'mare', 'cipher::secret', $3, $3
			)
		`, "cred-"+nodeID, nodeID, now); err != nil {
			t.Fatalf("insert nas credentials: %v", err)
		}
	case "CLOUD":
		providerPayload, err := json.Marshal(map[string]string{
			"cloudName":     "115",
			"cloudUserName": "mare-user",
			"cloudPath":     basePath,
		})
		if err != nil {
			t.Fatalf("marshal cloud payload: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
			) VALUES (
				$1, $1, $2, 'CLOUD', '115', $3, 'QR', '115 云归档', true, $4, $4
			)
		`, nodeID, "115 云归档", basePath, now); err != nil {
			t.Fatalf("insert cloud node: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO storage_node_credentials (
				id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
			) VALUES (
				$1, $2, 'TOKEN', 'cipher::token', 'tv', 'CONFIGURED', $3, $3
			)
		`, "cred-"+nodeID, nodeID, now); err != nil {
			t.Fatalf("insert cloud credentials: %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO cloud_node_profiles (
				id, storage_node_id, provider_vendor, auth_method, remote_root_path, provider_payload, last_auth_at, updated_at, created_at
			) VALUES (
				$1, $2, '115', 'QR', $3, $4::jsonb, $5, $5, $5
			)
		`, "profile-"+nodeID, nodeID, basePath, string(providerPayload), now); err != nil {
			t.Fatalf("insert cloud profile: %v", err)
		}
	default:
		t.Fatalf("unsupported node type %s", nodeType)
	}
}

func insertMountForRoute(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	libraryID string,
	mountID string,
	nodeID string,
	name string,
	nodeType string,
	basePath string,
	now time.Time,
) {
	t.Helper()

	mountSourceType := "LOCAL_PATH"
	switch nodeType {
	case "NAS":
		mountSourceType = "NAS_SHARE"
	case "CLOUD":
		mountSourceType = "CLOUD_FOLDER"
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			$1, $1, $2, '商业摄影资产库', $3, $4, $5, 'READ_WRITE',
			$6, '/', 'NEVER', 'MANUAL', true, 0, $7, $7
		)
	`, mountID, libraryID, nodeID, name, mountSourceType, basePath, now); err != nil {
		t.Fatalf("insert mount: %v", err)
	}
}

func buildReplicaPhysicalPath(nodeType string, basePath string, relativePath string) string {
	if nodeType == "CLOUD" {
		return "/" + strings.TrimLeft(strings.ReplaceAll(relativePath, "\\", "/"), "/")
	}
	return filepath.Join(basePath, filepath.FromSlash(relativePath))
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

func findItemByKey(t *testing.T, items []jobdto.ItemRecord, itemKey string) jobdto.ItemRecord {
	t.Helper()

	for _, item := range items {
		if item.ItemKey == itemKey {
			return item
		}
	}
	t.Fatalf("expected item %s", itemKey)
	return jobdto.ItemRecord{}
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
