package notifications_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	"mare/services/center/internal/issues"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/notifications"
	issuedto "mare/shared/contracts/dto/issue"
)

const developmentDatabaseURL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"

func TestServiceProjectsIssueAndJobNotificationsFromRealJobFlow(t *testing.T) {
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
	notificationService := notifications.NewService(pool)
	jobService.SetIssueSynchronizer(issueService)
	jobService.SetNotificationSynchronizer(notificationService)
	issueService.SetNotificationSynchronizer(notificationService)

	failFirstAttempt := true
	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		if failFirstAttempt {
			failFirstAttempt = false
			return fmt.Errorf("扫描目录失败")
		}
		return nil
	})
	jobService.Start(ctx)

	result, err := jobService.CreateJob(ctx, jobs.CreateJobInput{
		LibraryID:      ptr("photo"),
		JobFamily:      jobs.JobFamilyMaintenance,
		JobIntent:      jobs.JobIntentScanDirectory,
		Title:          "扫描目录：/2026/Shanghai Launch",
		Summary:        "用于验证通知切片",
		SourceDomain:   jobs.SourceDomainFileCenter,
		SourceSnapshot: map[string]any{"libraryName": "商业摄影资产库", "directoryId": "dir-launch", "path": "2026/Shanghai Launch"},
		Priority:       jobs.PriorityNormal,
		CreatedByType:  jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{
				ItemKey:    "dir:dir-launch",
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
		t.Fatalf("create job: %v", err)
	}

	waitForJobStatus(t, ctx, jobService, result.JobID, jobs.StatusFailed)
	notices := waitForNotificationCount(t, ctx, notificationService, 1)

	var issueNotice *notifications.Record
	var jobNotice *notifications.Record
	for i := range notices.Items {
		record := notices.Items[i]
		switch record.SourceType {
		case notifications.SourceTypeIssue:
			issueNotice = &record
		case notifications.SourceTypeJob:
			jobNotice = &record
		}
	}
	if issueNotice == nil {
		t.Fatalf("expected issue notice, got %+v", notices.Items)
	}
	if issueNotice.Kind != notifications.KindActionRequired {
		t.Fatalf("expected issue notice kind %s, got %+v", notifications.KindActionRequired, issueNotice)
	}
	if issueNotice.DefaultTargetKind != notifications.TargetIssues {
		t.Fatalf("expected issue notice target %s, got %+v", notifications.TargetIssues, issueNotice)
	}
	if issueNotice.JumpParams.TaskID == nil || *issueNotice.JumpParams.TaskID != result.JobID {
		t.Fatalf("expected issue notice task id %s, got %+v", result.JobID, issueNotice.JumpParams)
	}
	if issueNotice.JumpParams.FileNodeID == nil || *issueNotice.JumpParams.FileNodeID != "dir-launch" {
		t.Fatalf("expected issue notice file node id dir-launch, got %+v", issueNotice.JumpParams)
	}

	if jobNotice != nil {
		t.Fatalf("did not expect failed job reminder when issue is active, got %+v", *jobNotice)
	}

	activeIssues := waitForIssueCount(t, ctx, issueService, 1)
	if _, err := issueService.ApplyAction(ctx, issues.ActionRequest{
		IDs:    []string{activeIssues.Items[0].ID},
		Action: issues.ActionRetry,
	}); err != nil {
		t.Fatalf("retry issue: %v", err)
	}

	waitForJobStatus(t, ctx, jobService, result.JobID, jobs.StatusCompleted)
	waitForNoActiveIssues(t, ctx, issueService)
	if err := notificationService.SyncJobNotifications(ctx, result.JobID); err != nil {
		t.Fatalf("sync notifications after completion: %v", err)
	}
	remaining := waitForNotificationCount(t, ctx, notificationService, 2)
	updatedReminder := findNotificationBySourceType(remaining.Items, notifications.SourceTypeJob)
	if updatedReminder == nil {
		t.Fatalf("expected updated job reminder, got %+v", remaining.Items)
	}
	if updatedReminder.Kind != notifications.KindReminder {
		t.Fatalf("expected reminder notice after completion, got %+v", *updatedReminder)
	}
	if updatedReminder.Summary == issueNotice.Summary || !strings.Contains(updatedReminder.Summary, "作业已完成") {
		t.Fatalf("expected reminder summary to refresh after retry, got %+v", *updatedReminder)
	}
}

func TestServiceStreamsNotificationLifecycleEvents(t *testing.T) {
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

	jobService := jobs.NewService(pool)
	issueService := issues.NewService(pool, jobService)
	notificationService := notifications.NewService(pool)
	jobService.SetIssueSynchronizer(issueService)
	jobService.SetNotificationSynchronizer(notificationService)
	issueService.SetNotificationSynchronizer(notificationService)

	events, cancelStream := notificationService.Subscribe()
	defer cancelStream()

	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		return fmt.Errorf("扫描目录失败")
	})
	jobService.Start(ctx)

	_, err := jobService.CreateJob(ctx, jobs.CreateJobInput{
		JobFamily:     jobs.JobFamilyMaintenance,
		JobIntent:     jobs.JobIntentScanDirectory,
		Title:         "扫描目录：/events",
		Summary:       "用于验证通知事件流",
		SourceDomain:  jobs.SourceDomainFileCenter,
		Priority:      jobs.PriorityNormal,
		CreatedByType: jobs.CreatedByUser,
		Items: []jobs.CreateItemInput{
			{ItemKey: "dir:/events", ItemType: jobs.ItemTypeDirectoryScan, Title: "扫描目录：/events", Summary: "失败"},
		},
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	received := make([]notifications.StreamEvent, 0, 2)
	deadline := time.After(10 * time.Second)
	for len(received) < 2 {
		select {
		case event := <-events:
			received = append(received, event)
		case <-deadline:
			t.Fatalf("expected notification stream events, got %+v", received)
		}
	}

	if received[0].Topic != "NOTIFICATION" {
		t.Fatalf("expected notification topic, got %+v", received[0])
	}
	if received[0].EventType == "" {
		t.Fatalf("expected notification event type, got %+v", received[0])
	}
}

func waitForNotificationCount(t *testing.T, ctx context.Context, service *notifications.Service, expected int) notifications.ListResponse {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ListNotifications(ctx, notifications.ListQuery{Page: 1, PageSize: 20, IncludeStale: true})
		if err == nil && len(result.Items) == expected {
			return result
		}
		time.Sleep(100 * time.Millisecond)
	}

	result, err := service.ListNotifications(ctx, notifications.ListQuery{Page: 1, PageSize: 20, IncludeStale: true})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	t.Fatalf("expected %d notifications, got %+v", expected, result.Items)
	return result
}

func waitForVisibleNotificationCount(t *testing.T, ctx context.Context, service *notifications.Service, expected int) notifications.ListResponse {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ListNotifications(ctx, notifications.ListQuery{Page: 1, PageSize: 20})
		if err == nil && len(result.Items) == expected {
			return result
		}
		time.Sleep(100 * time.Millisecond)
	}

	result, err := service.ListNotifications(ctx, notifications.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list visible notifications: %v", err)
	}
	t.Fatalf("expected %d visible notifications, got %+v", expected, result.Items)
	return result
}

func findNotificationBySourceType(items []notifications.Record, sourceType string) *notifications.Record {
	for i := range items {
		if items[i].SourceType == sourceType {
			return &items[i]
		}
	}
	return nil
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
		result, err := service.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
		if err == nil && countActiveIssues(result) == 0 {
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

func countActiveIssues(result issuedto.ListResponse) int {
	count := 0
	for _, item := range result.Items {
		switch item.Status {
		case "待处理", "待确认", "处理中":
			count++
		}
	}
	return count
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

	schema := fmt.Sprintf("test_notifications_%d", time.Now().UnixNano())
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
		DROP TABLE IF EXISTS notifications;
		DROP TABLE IF EXISTS issue_object_links;
		DROP TABLE IF EXISTS issue_events;
		DROP TABLE IF EXISTS issues;
		DROP TABLE IF EXISTS job_object_links;
		DROP TABLE IF EXISTS job_events;
		DROP TABLE IF EXISTS job_attempts;
		DROP TABLE IF EXISTS job_items;
		DROP TABLE IF EXISTS jobs;
		DROP TABLE IF EXISTS libraries;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

func ptr(value string) *string {
	return &value
}
