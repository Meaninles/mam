package governance_test

import (
	"context"
	"errors"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	"mare/services/center/internal/governance"
	"mare/services/center/internal/issues"
	"mare/services/center/internal/jobs"
	"mare/services/center/internal/notifications"
	jobdto "mare/shared/contracts/dto/job"
)

const governanceDevelopmentDatabaseURL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"

func TestGovernanceServiceCreatesScheduledMountScanJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openGovernanceTestPool(t, ctx)
	defer pool.Close()
	resetGovernanceSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at)
		VALUES ('library-1', 'library-1', '资料库', '/', 'ACTIVE', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO library_directories (
			id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
		) VALUES (
			'dir-root-library-1', 'library-1', '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $1, $1
		)
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
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-1', 'mount-1', 'library-1', '资料库', 'local-node-1', '待扫描挂载', 'LOCAL_PATH', 'READ_ONLY',
			'C:/mare/assets', '/', 'NEVER', 'SCHEDULED', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, next_scan_at, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-1', 'mount-1', 'IDLE', $1, 'NOT_REQUIRED', 'ONLINE', $2, $2
		)
	`, now.Add(-time.Minute), now); err != nil {
		t.Fatalf("insert mount runtime: %v", err)
	}

	jobService := jobs.NewService(pool)
	jobService.RegisterExecutor(jobs.JobIntentScanDirectory, func(ctx context.Context, execution jobs.ExecutionContext) error {
		return nil
	})
	jobService.Start(ctx)

	service := governance.NewService(pool, governance.Options{
		JobService:            jobService,
		ScheduledScanInterval: time.Hour,
		TickInterval:          time.Second,
	})
	service.SetNow(func() time.Time { return now })

	if err := service.Tick(ctx); err != nil {
		t.Fatalf("tick governance: %v", err)
	}

	detail := waitForGovernanceJobStatus(t, ctx, jobService, jobs.StatusCompleted)
	if detail.Job.SourceDomain != jobs.SourceDomainScheduled {
		t.Fatalf("expected source domain %s, got %+v", jobs.SourceDomainScheduled, detail.Job)
	}
	if len(detail.Items) != 1 || detail.Items[0].ItemKey != "mount:mount-1" {
		t.Fatalf("unexpected scheduled scan items: %+v", detail.Items)
	}

	var nextScanAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT next_scan_at FROM mount_runtime WHERE mount_id = 'mount-1'`).Scan(&nextScanAt); err != nil {
		t.Fatalf("query next_scan_at: %v", err)
	}
	if nextScanAt == nil || !nextScanAt.After(now) {
		t.Fatalf("expected next_scan_at advanced after scheduling, got %v", nextScanAt)
	}

	var jobCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE source_domain = $1`, jobs.SourceDomainScheduled).Scan(&jobCount); err != nil {
		t.Fatalf("count scheduled jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("expected exactly one scheduled job, got %d", jobCount)
	}

	if err := service.Tick(ctx); err != nil {
		t.Fatalf("tick governance again: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE source_domain = $1`, jobs.SourceDomainScheduled).Scan(&jobCount); err != nil {
		t.Fatalf("count scheduled jobs after second tick: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("expected no duplicate scheduled jobs, got %d", jobCount)
	}

	logs, err := service.ListAuditLogs(ctx)
	if err != nil {
		t.Fatalf("list governance audit logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected governance audit logs after scheduled scan tick")
	}
}

func TestGovernanceServiceCreatesHeartbeatCheckJobAndPublishesFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := openGovernanceTestPool(t, ctx)
	defer pool.Close()
	resetGovernanceSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO libraries (id, code, name, root_label, status, created_at, updated_at)
		VALUES ('library-1', 'library-1', '资料库', '/', 'ACTIVE', $1, $1)
	`, now); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO library_directories (
			id, library_id, relative_path, name, parent_path, depth, source_kind, status, sort_name, created_at, updated_at
		) VALUES (
			'dir-root-library-1', 'library-1', '/', '/', NULL, 0, 'MANUAL', 'ACTIVE', '/', $1, $1
		)
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
		INSERT INTO mounts (
			id, code, library_id, library_name, storage_node_id, name, mount_source_type, mount_mode,
			source_path, relative_root_path, heartbeat_policy, scan_policy, enabled, sort_order, created_at, updated_at
		) VALUES (
			'mount-1', 'mount-1', 'library-1', '资料库', 'local-node-1', '待巡检挂载', 'LOCAL_PATH', 'READ_ONLY',
			'C:/mare/assets', '/', 'HOURLY', 'MANUAL', true, 0, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert mount: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mount_runtime (
			id, mount_id, scan_status, next_heartbeat_at, auth_status, health_status, created_at, updated_at
		) VALUES (
			'mount-runtime-1', 'mount-1', 'IDLE', $1, 'NOT_REQUIRED', 'ONLINE', $2, $2
		)
	`, now.Add(-time.Minute), now); err != nil {
		t.Fatalf("insert mount runtime: %v", err)
	}

	jobService := jobs.NewService(pool)
	jobService.SetRetryBaseDelay(100 * time.Millisecond)
	issueService := issues.NewService(pool, jobService)
	notificationService := notifications.NewService(pool)
	jobService.SetIssueSynchronizer(issueService)
	jobService.SetNotificationSynchronizer(notificationService)
	issueService.SetNotificationSynchronizer(notificationService)
	jobService.RegisterExecutor(jobs.JobIntentConnectionTest, func(ctx context.Context, execution jobs.ExecutionContext) error {
		return errors.New("连接测试失败：根目录不可达")
	})
	jobService.Start(ctx)

	service := governance.NewService(pool, governance.Options{
		JobService:            jobService,
		ScheduledScanInterval: time.Hour,
		TickInterval:          time.Second,
	})
	service.SetNow(func() time.Time { return now })

	if err := service.Tick(ctx); err != nil {
		t.Fatalf("tick governance: %v", err)
	}

	detail := waitForGovernanceJobStatusByIntent(t, ctx, jobService, jobs.JobIntentConnectionTest, jobs.StatusFailed)
	if detail.Job.JobIntent != jobs.JobIntentConnectionTest {
		t.Fatalf("expected connection test job, got %+v", detail.Job)
	}
	if detail.Job.SourceDomain != jobs.SourceDomainScheduled {
		t.Fatalf("expected scheduled source domain, got %+v", detail.Job)
	}

	var nextHeartbeatAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT next_heartbeat_at FROM mount_runtime WHERE mount_id = 'mount-1'`).Scan(&nextHeartbeatAt); err != nil {
		t.Fatalf("query next_heartbeat_at: %v", err)
	}
	if nextHeartbeatAt == nil || !nextHeartbeatAt.After(now) {
		t.Fatalf("expected next_heartbeat_at advanced after heartbeat scheduling, got %v", nextHeartbeatAt)
	}

	issuesList, err := issueService.ListIssues(ctx, issues.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(issuesList.Items) == 0 {
		t.Fatalf("expected heartbeat failure to create issue")
	}

	notices, err := notificationService.ListNotifications(ctx, notifications.ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(notices.Items) == 0 {
		t.Fatalf("expected heartbeat failure to create notification")
	}
}

func TestGovernanceServiceTickTriggersMissingReplicaSync(t *testing.T) {
	service := governance.NewService(nil, governance.Options{})
	var calls atomic.Int32
	service.SetMissingReplicaSyncer(fakeMissingReplicaSyncer{calls: &calls})

	if err := service.Tick(context.Background()); err != nil {
		t.Fatalf("tick governance: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected missing replica sync to run once, got %d", calls.Load())
	}
}

func waitForGovernanceJobStatus(t *testing.T, ctx context.Context, service *jobs.Service, expected string) jobdto.Detail {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ListJobs(ctx, jobs.ListQuery{Page: 1, PageSize: 20, SourceDomain: jobs.SourceDomainScheduled})
		if err == nil && len(result.Items) > 0 {
			detail, detailErr := service.LoadJobDetail(ctx, result.Items[0].ID)
			if detailErr == nil && detail.Job.Status == expected {
				return detail
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for scheduled job status %s", expected)
	return jobdto.Detail{}
}

func waitForGovernanceJobStatusByIntent(t *testing.T, ctx context.Context, service *jobs.Service, intent string, expected string) jobdto.Detail {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := service.ListJobs(ctx, jobs.ListQuery{Page: 1, PageSize: 20, SourceDomain: jobs.SourceDomainScheduled})
		if err == nil {
			for _, item := range result.Items {
				if item.JobIntent != intent {
					continue
				}
				detail, detailErr := service.LoadJobDetail(ctx, item.ID)
				if detailErr == nil && detail.Job.Status == expected {
					return detail
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for governance job %s status %s", intent, expected)
	return jobdto.Detail{}
}

func openGovernanceTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := governanceTestDSN(t, ctx)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	return pool
}

func governanceTestDSN(t *testing.T, ctx context.Context) string {
	t.Helper()

	adminPool, err := pgxpool.New(ctx, governanceDevelopmentDatabaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	schema := "governance_test_" + time.Now().UTC().Format("20060102150405.000000000")
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
	})

	parsed, err := url.Parse(governanceDevelopmentDatabaseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func resetGovernanceSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS notifications;
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
		DROP TABLE IF EXISTS cloud_node_profiles;
		DROP TABLE IF EXISTS storage_node_runtime;
		DROP TABLE IF EXISTS storage_node_credentials;
		DROP TABLE IF EXISTS storage_nodes;
		DROP TABLE IF EXISTS agents;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

type fakeMissingReplicaSyncer struct {
	calls *atomic.Int32
}

func (f fakeMissingReplicaSyncer) SyncMissingReplicaIssues(context.Context) error {
	f.calls.Add(1)
	return nil
}
